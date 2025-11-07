//go:build linux
// +build linux

package staticpod

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/k3s-io/k3s/pkg/agent/containerd"
	"github.com/k3s-io/k3s/pkg/agent/cri"
	"github.com/k3s-io/k3s/pkg/agent/cridockerd"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/signals"
	"github.com/k3s-io/k3s/pkg/spegel"
	"github.com/k3s-io/k3s/pkg/util"
	pkgerrors "github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/auth"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/logging"
	"github.com/rancher/rke2/pkg/podtemplate"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
	"k8s.io/kubernetes/pkg/util/hash"
	"sigs.k8s.io/yaml"
)

type StaticPodConfig struct {
	podtemplate.Config

	stopKubelet       context.CancelFunc
	CloudProvider     *CloudProviderConfig
	RuntimeEndpoint   string
	ManifestsDir      string
	IngressController string
	AuditPolicyFile   string
	PSAConfigFile     string
	KubeletPath       string
	ProfileMode       ProfileMode
	DisableETCD       bool
	ExternalDatabase  bool
	IsServer          bool

	apiServerReady <-chan struct{}
	etcdReady      chan struct{}
	criReady       chan struct{}
	dataReady      chan struct{}
}

// explicit interface check
var _ executor.Executor = &StaticPodConfig{}

type CloudProviderConfig struct {
	Name string
	Path string
}

// apiserverSyncAndReady returns a channel that is closed once the etcd and apiserver static pods have been synced,
// and the apiserver readyz endpoint returns success.
func apiserverSyncAndReady(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) <-chan struct{} {
	ready := make(chan struct{})
	go func() {
		defer close(ready)
		reconcileStaticPods(ctx, cfg.ContainerRuntimeEndpoint, cfg.DataDir)
		<-util.APIServerReadyChan(ctx, nodeConfig.AgentConfig.KubeConfigK3sController, util.DefaultAPIServerReadyTimeout)
	}()
	return ready
}

// Bootstrap prepares the static executor to run components by setting the system default registry
// and staging the kubelet and containerd binaries.  On servers, it also ensures that manifests are
// copied in to place and in sync with the system configuration.
func (s *StaticPodConfig) Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	s.apiServerReady = apiserverSyncAndReady(ctx, nodeConfig, cfg)
	s.etcdReady = make(chan struct{})
	s.criReady = make(chan struct{})
	s.dataReady = make(chan struct{})

	// On servers this is set to an initial value from the CLI when the resolver is created, so that
	// static pod manifests can be created before the agent bootstrap is complete. The agent itself
	// really only needs to know about the runtime and pause images, all of which are configured after the
	// default registry has been set by the server.
	if nodeConfig.AgentConfig.SystemDefaultRegistry != "" {
		if err := s.Resolver.ParseAndSetDefaultRegistry(nodeConfig.AgentConfig.SystemDefaultRegistry); err != nil {
			return err
		}
	}
	pauseImage, err := s.Resolver.GetReference(images.Pause)
	if err != nil {
		return err
	}
	nodeConfig.AgentConfig.PauseImage = pauseImage.Name()

	if binDir, err := bootstrap.BinDir(s.Resolver, cfg); err != nil {
		return err
	} else {
		os.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	}

	// stage bootstrap content from runtime image, and close the dataReady channel when successful
	go func() {
		if err := s.stageData(ctx, nodeConfig, cfg); err != nil {
			signals.RequestShutdown(err)
		} else {
			close(s.dataReady)
		}
	}()

	// Remove the kube-proxy static pod manifest before starting the agent.
	// If kube-proxy should run, the manifest will be recreated after the apiserver is up.
	if err := s.removeTemplate("kube-proxy"); err != nil {
		logrus.Error(err)
	}

	return nil
}

// Kubelet starts the kubelet in a subprocess with watching goroutine.
func (s *StaticPodConfig) Kubelet(ctx context.Context, args []string) error {
	extraArgs := []string{
		"--volume-plugin-dir=/var/lib/kubelet/volumeplugins",
		"--file-check-frequency=5s",
		"--sync-frequency=30s",
	}
	if s.CloudProvider != nil {
		extraArgs = append(extraArgs,
			"--cloud-provider="+s.CloudProvider.Name,
		)
	}

	args = append(extraArgs, args...)
	args, logOut := logging.ExtractFromArgs(args)
	ctx, cancel := context.WithCancel(ctx)
	s.stopKubelet = cancel

	go func() {
		wait.PollUntilContextCancel(ctx, 5*time.Second, true, func(ctx context.Context) (bool, error) {
			cmd := exec.CommandContext(ctx, s.KubeletPath, args...)
			cmd.Stdout = logOut
			cmd.Stderr = logOut
			addDeathSig(cmd)

			err := cmd.Run()
			logrus.Errorf("Kubelet exited: %v", err)

			return false, nil
		})
	}()

	return nil
}

// KubeProxy starts Kube Proxy as a static pod.
func (s *StaticPodConfig) KubeProxy(_ context.Context, args []string) error {
	podSpec, err := s.Config.KubeProxy(args)
	if err != nil {
		return err
	}

	podSpec.Privileged = true
	podSpec.HostNetwork = true

	return s.writeTemplate(podSpec)
}

// APIServerHandlers returning the authenticator and request handler for requests to the apiserver endpoint.
func (s *StaticPodConfig) APIServerHandlers(ctx context.Context) (authenticator.Request, http.Handler, error) {
	<-s.APIServerReadyChan()
	kubeConfigAPIServer := filepath.Join(s.DataDir, "server", "cred", "api-server.kubeconfig")
	tokenauth, err := auth.BootstrapTokenAuthenticator(ctx, kubeConfigAPIServer)
	return tokenauth, http.NotFoundHandler(), err
}

// APIServer sets up the apiserver static pod once etcd is available.
func (s *StaticPodConfig) APIServer(_ context.Context, args []string) error {
	if err := s.removeTemplate("kube-apiserver"); err != nil {
		return err
	}

	auditLogFile := ""
	kubeletPreferredAddressTypesFound := false
	for i, arg := range args {
		switch name, value, _ := strings.Cut(arg, "="); name {
		case "--advertise-port", "--basic-auth-file":
			// This is an option k3s adds that does not exist upstream
			args = append(args[:i], args[i+1:]...)
		case "--etcd-servers":
			if s.ExternalDatabase {
				args = append(args[:i], args[i+1:]...)
				args = append([]string{"--etcd-servers=" + "unixs://" + filepath.Join(s.DataDir, "server", "kine.sock")}, args...)
			}
		case "--audit-log-path":
			auditLogFile = value
		case "--kubelet-preferred-address-types":
			kubeletPreferredAddressTypesFound = true
		}
	}
	if !kubeletPreferredAddressTypesFound {
		args = append([]string{"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname"}, args...)
	}

	if s.ProfileMode.isCISMode() {
		args = append([]string{"--service-account-extend-token-expiration=false"}, args...)
		if s.AuditPolicyFile == "" {
			s.AuditPolicyFile = podtemplate.DefaultAuditPolicyFile
		}
	}

	if s.AuditPolicyFile != "" {
		if err := podtemplate.WriteDefaultPolicyFile(s.AuditPolicyFile); err != nil {
			return err
		}
		extraArgs := []string{
			"--audit-policy-file=" + s.AuditPolicyFile,
			"--audit-log-maxage=30",
			"--audit-log-maxbackup=10",
			"--audit-log-maxsize=100",
		}
		if auditLogFile == "" {
			auditLogFile = filepath.Join(s.DataDir, "server/logs/audit.log")
			extraArgs = append(extraArgs, "--audit-log-path="+auditLogFile)
		}
		args = append(extraArgs, args...)
	}

	args = append([]string{"--admission-control-config-file=" + s.PSAConfigFile}, args...)

	files := []string{}
	excludeFiles := []string{}
	if !s.DisableETCD {
		files = append(files, etcdNameFile(s.DataDir))
	}

	dirs := podtemplate.OnlyExisting(podtemplate.SSLDirs)
	if auditLogFile != "" && auditLogFile != "-" {
		dirs = append(dirs, filepath.Dir(auditLogFile))
		excludeFiles = append(excludeFiles, auditLogFile)
	}

	// Need to mount the entire server directory so that any files recreated in this directory
	// after the pod has been started are not masked by a stale mount
	// encryption config is refreshed by the secrets-encryption controller
	// so we mount the directory to allow the pod to see the updates
	dirs = append(dirs, filepath.Join(s.DataDir, "server"))
	excludeFiles = append(excludeFiles, filepath.Join(s.DataDir, "server/cred/encryption-config.json"))

	podSpec, err := s.Config.APIServer(args)
	if err != nil {
		return err
	}

	podSpec.Files = files
	podSpec.Dirs = dirs
	podSpec.ExcludeFiles = excludeFiles
	podSpec.HostNetwork = true

	return podtemplate.After(s.ETCDReadyChan(), func() error {
		return s.writeTemplate(podSpec)
	})
}

var permitPortSharingFlag = []string{"--permit-port-sharing=true"}

// Scheduler starts the kube-scheduler static pod, once the apiserver is available.
func (s *StaticPodConfig) Scheduler(_ context.Context, nodeReady <-chan struct{}, args []string) error {
	files := []string{}
	if !s.DisableETCD {
		files = []string{etcdNameFile(s.DataDir)}
	}

	podSpec, err := s.Config.Scheduler(append(permitPortSharingFlag, args...))
	if err != nil {
		return err
	}

	podSpec.Files = files
	podSpec.HostNetwork = true

	return podtemplate.After(s.APIServerReadyChan(), func() error {
		return s.writeTemplate(podSpec)
	})
}

// ControllerManager starts the kube-controller-manager static pod, once the apiserver is available.
func (s *StaticPodConfig) ControllerManager(_ context.Context, args []string) error {
	if s.CloudProvider != nil {
		extraArgs := []string{
			"--cloud-provider=" + s.CloudProvider.Name,
			"--cloud-config=" + s.CloudProvider.Path,
		}
		args = append(extraArgs, args...)
	}

	extraArgs := []string{
		"--flex-volume-plugin-dir=/var/lib/kubelet/volumeplugins",
		"--terminated-pod-gc-threshold=1000",
	}
	args = append(extraArgs, args...)
	args = append(permitPortSharingFlag, args...)

	files := []string{}
	if !s.DisableETCD {
		files = []string{etcdNameFile(s.DataDir)}
	}

	podSpec, err := s.Config.ControllerManager(args)
	if err != nil {
		return err
	}

	podSpec.Files = files
	podSpec.Dirs = podtemplate.OnlyExisting(podtemplate.SSLDirs)
	podSpec.HostNetwork = true

	return podtemplate.After(s.APIServerReadyChan(), func() error {
		return s.writeTemplate(podSpec)
	})
}

// CloudControllerManager starts the cloud-controller-manager static pod, once the cloud controller manager RBAC
// (and subsequently, the api server) is available.
func (s *StaticPodConfig) CloudControllerManager(_ context.Context, ccmRBACReady <-chan struct{}, args []string) error {
	podSpec, err := s.Config.CloudControllerManager(args)
	if err != nil {
		return err
	}

	podSpec.Dirs = podtemplate.OnlyExisting(podtemplate.SSLDirs)
	podSpec.HostNetwork = true

	return podtemplate.After(ccmRBACReady, func() error {
		return s.writeTemplate(podSpec)
	})
}

// CurrentETCDOptions retrieves the etcd configuration from the static pod definition at etcd.yaml
// in the manifests directory, if it exists.
func (s *StaticPodConfig) CurrentETCDOptions() (opts executor.InitialOptions, err error) {
	bytes, err := ioutil.ReadFile(filepath.Join(s.ManifestsDir, "etcd.yaml"))
	if os.IsNotExist(err) {
		return opts, nil
	}

	pod := &v1.Pod{}
	if err := yaml.Unmarshal(bytes, pod); err != nil {
		return opts, err
	}

	v, ok := pod.Annotations["etcd.k3s.io/initial"]
	if ok {
		return opts, json.NewDecoder(strings.NewReader(v)).Decode(&opts)
	}

	return opts, nil
}

// ETCD starts the etcd static pod.
func (s *StaticPodConfig) ETCD(ctx context.Context, wg *sync.WaitGroup, args *executor.ETCDConfig, extraArgs []string, test executor.TestFunc) error {
	go func() {
		for {
			if err := test(ctx, true); err != nil {
				logrus.Infof("Failed to test etcd connection: %v", err)
			} else {
				logrus.Info("Connection to etcd is ready")
				close(s.etcdReady)
				return
			}

			select {
			case <-time.After(5 * time.Second):
			case <-ctx.Done():
				return
			}
		}
	}()

	// nil args indicates a no-op start; all we need to do is wait for the test
	// func to indicate readiness and close the channel.
	if args == nil {
		return nil
	}

	initial, err := json.Marshal(args.InitialOptions)
	if err != nil {
		return err
	}

	confFile, err := args.ToConfigFile(extraArgs)
	if err != nil {
		return err
	}

	podSpec, err := s.Config.ETCD([]string{"--config-file=" + confFile})
	if err != nil {
		return err
	}

	podSpec.Annotations = map[string]string{"etcd.k3s.io/initial": string(initial)}
	podSpec.Dirs = []string{args.DataDir}
	podSpec.Files = []string{
		args.ServerTrust.CertFile,
		args.ServerTrust.KeyFile,
		args.ServerTrust.TrustedCAFile,
		args.PeerTrust.CertFile,
		args.PeerTrust.KeyFile,
		args.PeerTrust.TrustedCAFile,
	}
	podSpec.HostNetwork = true

	if s.ProfileMode.isAnyMode() {
		etcdUser, err := user.Lookup("etcd")
		if err != nil {
			return err
		}
		uid, err := strconv.ParseInt(etcdUser.Uid, 10, 64)
		if err != nil {
			return err
		}
		gid, err := strconv.ParseInt(etcdUser.Gid, 10, 64)
		if err != nil {
			return err
		}
		if podSpec.SecurityContext == nil {
			podSpec.SecurityContext = &v1.PodSecurityContext{}
		}
		podSpec.SecurityContext.RunAsUser = &uid
		podSpec.SecurityContext.RunAsGroup = &gid

		for _, p := range append(podSpec.Dirs, podSpec.Files...) {
			if err := chownr(p, int(uid), int(gid)); err != nil {
				return err
			}
		}
	}

	if cmds.AgentConfig.EnableSELinux {
		if podSpec.SecurityContext == nil {
			podSpec.SecurityContext = &v1.PodSecurityContext{}
		}
		if podSpec.SecurityContext.SELinuxOptions == nil {
			podSpec.SecurityContext.SELinuxOptions = &v1.SELinuxOptions{
				Type: "rke2_service_db_t",
			}
		}
	}

	// If performing a cluster-reset, ensure that the kubelet and etcd are stopped when the context is cancelled at the end of the cluster-reset process.
	if args.ForceNewCluster {
		go func() {
			<-ctx.Done()
			logrus.Infof("Shutting down kubelet and etcd")
			if s.stopKubelet != nil {
				s.stopKubelet()
			}
			if err := s.stopEtcd(); err != nil {
				logrus.Errorf("Failed to stop etcd: %v", err)
			}
		}()
	}

	return s.writeTemplate(podSpec)
}

// Containerd starts the k3s implementation of containerd
func (s *StaticPodConfig) Containerd(ctx context.Context, cfg *daemonconfig.Node) error {
	<-s.dataReadyChan()
	return executor.CloseIfNilErr(containerd.Run(ctx, cfg), s.criReady)
}

// Docker starts the k3s implementation of cridockerd
func (s *StaticPodConfig) Docker(ctx context.Context, cfg *daemonconfig.Node) error {
	<-s.dataReadyChan()
	return executor.CloseIfNilErr(cridockerd.Run(ctx, cfg), s.criReady)
}

func (s *StaticPodConfig) CRI(ctx context.Context, cfg *daemonconfig.Node) error {
	<-s.dataReadyChan()
	// agentless sets cri socket path to /dev/null to indicate no CRI is needed
	if cfg.ContainerRuntimeEndpoint != "/dev/null" {
		return executor.CloseIfNilErr(cri.WaitForService(ctx, cfg.ContainerRuntimeEndpoint, "CRI"), s.criReady)
	}
	return executor.CloseIfNilErr(nil, s.criReady)
}

func (s *StaticPodConfig) APIServerReadyChan() <-chan struct{} {
	if s.apiServerReady == nil {
		panic("executor not bootstrapped")
	}
	return s.apiServerReady
}

func (s *StaticPodConfig) ETCDReadyChan() <-chan struct{} {
	if s.etcdReady == nil {
		panic("executor not bootstrapped")
	}
	return s.etcdReady
}

func (s *StaticPodConfig) CRIReadyChan() <-chan struct{} {
	if s.criReady == nil {
		panic("executor not bootstrapped")
	}
	return s.criReady
}

func (s *StaticPodConfig) dataReadyChan() <-chan struct{} {
	if s.dataReady == nil {
		panic("executor not bootstrapped")
	}
	return s.dataReady
}

func (s *StaticPodConfig) IsSelfHosted() bool {
	return true
}

// stopEtcd searches the container runtime endpoint for the etcd static pod, and terminates it.
func (s *StaticPodConfig) stopEtcd() error {
	ctx := context.Background()
	conn, err := cri.Connection(ctx, s.RuntimeEndpoint)
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to connect to cri")
	}
	cRuntime := runtimeapi.NewRuntimeServiceClient(conn)
	defer conn.Close()

	filter := &runtimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{
			"component":                   "etcd",
			"io.kubernetes.pod.namespace": "kube-system",
			"tier":                        "control-plane",
		},
	}
	resp, err := cRuntime.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{Filter: filter})
	if err != nil {
		return pkgerrors.WithMessage(err, "failed to list pods")
	}

	for _, pod := range resp.Items {
		if pod.Annotations["kubernetes.io/config.source"] != "file" {
			continue
		}
		if _, err := cRuntime.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{PodSandboxId: pod.Id}); err != nil {
			return pkgerrors.WithMessage(err, "failed to terminate pod")
		}
	}

	return nil
}

// removeTemplate cleans up the static pod manifest for the given command from the specified directory.
// It does not actually stop or remove the static pod from the container runtime.
func (s *StaticPodConfig) removeTemplate(command string) error {
	manifestPath := filepath.Join(s.ManifestsDir, command+".yaml")
	if err := os.Remove(manifestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return pkgerrors.WithMessagef(err, "failed to remove %s static pod manifest", command)
	}
	logrus.Infof("Removed %s static pod manifest", command)
	return nil
}

// Writes a static pod manifest for the given pod template into the specified directory.
// This step also injects SecurityContext options and adds file mounts for any container args.
// Note that this does not actually run the command; the kubelet is responsible for picking up
// the manifest and creating container to run it.
func (s *StaticPodConfig) writeTemplate(spec *podtemplate.Spec) error {
	if spec == nil {
		return nil
	}

	if cmds.AgentConfig.EnableSELinux {
		if spec.SecurityContext == nil {
			spec.SecurityContext = &v1.PodSecurityContext{}
		}
		if spec.SecurityContext.SELinuxOptions == nil {
			spec.SecurityContext.SELinuxOptions = &v1.SELinuxOptions{
				Type: "rke2_service_t",
			}
		}
	}
	files, err := podtemplate.ReadFiles(spec.Args, spec.ExcludeFiles)
	if err != nil {
		return err
	}

	// TODO Check to make sure we aren't double mounting directories and the files in those directories

	spec.Files = append(spec.Files, files...)
	pod, err := podtemplate.Pod(spec)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(s.ManifestsDir, spec.Command+".yaml")

	// We hash the completed pod manifest use that as the UID; this mimics what upstream does:
	// https://github.com/kubernetes/kubernetes/blob/v1.24.0/pkg/kubelet/config/common.go#L58-68
	// We manually terminate static pods with incorrect UIDs, as the kubelet cannot be relied
	// upon to clean up the old one while the apiserver is down.
	// See https://github.com/rancher/rke2/issues/3387 and https://github.com/rancher/rke2/issues/3725
	hasher := md5.New()
	hash.DeepHashObject(hasher, pod)
	fmt.Fprintf(hasher, "file:%s", manifestPath)
	pod.UID = types.UID(hex.EncodeToString(hasher.Sum(nil)[0:]))

	b, err := yaml.Marshal(pod)
	if err != nil {
		return err
	}
	if s.ProfileMode.isAnyMode() {
		return writeFile(manifestPath, b, 0600)
	}
	return writeFile(manifestPath, b, 0644)
}

func (s *StaticPodConfig) stageData(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	// if spegel is enabled, wait for it to start up so that we can attempt to pull content through it
	if nodeConfig.EmbeddedRegistry && spegel.DefaultRegistry != nil {
		if err := wait.PollUntilContextTimeout(ctx, time.Second, time.Minute, true, func(ctx context.Context) (bool, error) {
			_, err := spegel.DefaultRegistry.Bootstrapper.Get(ctx)
			return err == nil, nil
		}); err != nil {
			return pkgerrors.WithMessage(err, "failed to wait for embedded registry")
		}
	}
	if err := bootstrap.Stage(ctx, s.Resolver, nodeConfig, cfg); err != nil {
		return err
	}
	if s.IsServer {
		return bootstrap.UpdateManifests(s.Resolver, s.IngressController, nodeConfig, cfg)
	}
	return nil
}

func writeFile(dest string, content []byte, perm fs.FileMode) error {
	name := filepath.Base(dest)
	dir := filepath.Dir(dest)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	existing, err := ioutil.ReadFile(dest)
	if err == nil && bytes.Equal(existing, content) {
		return nil
	}

	// Create the new manifest in a temporary directory up one level from the static pods dir and then
	// rename it into place.  Creating manifests in the destination directory risks the Kubelet
	// picking them up when they're partially written, or creating duplicate pods if it picks it up
	// before the temp file is renamed over the existing file.
	tmpdir, err := os.MkdirTemp(filepath.Dir(dir), name)
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	tmp := filepath.Join(tmpdir, name)
	if err := os.WriteFile(tmp, content, perm); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// chownr recursively changes the ownership of the given
// path to the given user ID and group ID.
func chownr(path string, uid, gid int) error {
	return filepath.Walk(path, func(name string, info os.FileInfo, err error) error {
		if err == nil {
			err = os.Chown(name, uid, gid)
		}
		return err
	})
}

func kineSock(dataDir string) string {
	return filepath.Join(dataDir, "server", "kine.sock")
}

func etcdNameFile(dataDir string) string {
	return filepath.Join(dataDir, "server", "db", "etcd", "name")
}
