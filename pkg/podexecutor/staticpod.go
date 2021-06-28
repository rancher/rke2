package podexecutor

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/cli/cmds"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/rke2/pkg/auth"
	"github.com/rancher/rke2/pkg/bootstrap"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/staticpod"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"sigs.k8s.io/yaml"
)

var (
	ssldirs = []string{
		"/etc/ssl/certs",
		"/etc/pki/tls/certs",
		"/etc/ca-certificates",
		"/usr/local/share/ca-certificates",
		"/usr/share/ca-certificates",
	}
)

type StaticPodConfig struct {
	ManifestsDir    string
	ImagesDir       string
	Resolver        *images.Resolver
	CloudProvider   *CloudProviderConfig
	CISMode         bool
	DataDir         string
	AuditPolicyFile string
	KubeletPath     string
	DisableETCD     bool
	IsServer        bool
}

type CloudProviderConfig struct {
	Name string
	Path string
}

// Bootstrap prepares the static executor to run components by setting the system default registry
// and staging the kubelet and containerd binaries.  On servers, it also ensures that manifests are
// copied in to place and in sync with the system configuration.
func (s *StaticPodConfig) Bootstrap(ctx context.Context, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
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

	// stage bootstrap content from runtime image
	execPath, err := bootstrap.Stage(s.Resolver, nodeConfig, cfg)
	if err != nil {
		return err
	}
	if err := os.Setenv("PATH", execPath+":"+os.Getenv("PATH")); err != nil {
		return err
	}
	if s.IsServer {
		return bootstrap.UpdateManifests(s.Resolver, nodeConfig, cfg)
	}
	return nil
}

// Kubelet starts the kubelet in a subprocess with watching goroutine.
func (s *StaticPodConfig) Kubelet(args []string) error {
	extraArgs := []string{
		"--volume-plugin-dir=/var/lib/kubelet/volumeplugins",
		"--file-check-frequency=5s",
		"--sync-frequency=30s",
	}
	if s.CloudProvider != nil {
		extraArgs = append(extraArgs,
			"--cloud-provider="+s.CloudProvider.Name,
			"--cloud-config="+s.CloudProvider.Path,
		)
	}
	args = append(extraArgs, args...)
	go func() {
		for {
			cmd := exec.Command(s.KubeletPath, args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			addDeathSig(cmd)

			err := cmd.Run()
			logrus.Errorf("Kubelet exited: %v", err)

			time.Sleep(5 * time.Second)
		}
	}()

	return nil
}

// KubeProxy panics if used. KubeProxy for RKE2 is provided by a packaged component (rke2-kube-proxy Helm chart).
func (s *StaticPodConfig) KubeProxy(args []string) error {
	panic("kube-proxy unsupported")
}

// APIServer sets up the apiserver static pod once etcd is available, returning the authenticator and request handler.
func (s *StaticPodConfig) APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) (authenticator.Request, http.Handler, error) {
	image, err := s.Resolver.GetReference(images.KubeAPIServer)
	if err != nil {
		return nil, nil, err
	}
	if err := images.Pull(s.ImagesDir, images.KubeAPIServer, image); err != nil {
		return nil, nil, err
	}

	args = append([]string{"--kubelet-preferred-address-types=InternalIP,ExternalIP,Hostname"}, args...)
	auditLogFile := filepath.Join(s.DataDir, "server/logs/audit.log")
	if s.CloudProvider != nil {
		extraArgs := []string{
			"--cloud-provider=" + s.CloudProvider.Name,
			"--cloud-config=" + s.CloudProvider.Path,
		}
		args = append(extraArgs, args...)
	}
	if s.CISMode {
		extraArgs := []string{
			"--audit-policy-file=" + s.AuditPolicyFile,
			"--audit-log-path=" + auditLogFile,
			"--audit-log-maxage=30",
			"--audit-log-maxbackup=10",
			"--audit-log-maxsize=100",
		}
		args = append(extraArgs, args...)
		if err := writeDefaultPolicyFile(s.AuditPolicyFile); err != nil {
			return nil, nil, err
		}
	}
	auth, err := auth.FromArgs(args)
	for i, arg := range args {
		// This is an option k3s adds that does not exist upstream
		if strings.HasPrefix(arg, "--advertise-port=") {
			args = append(args[:i], args[i+1:]...)
		}
		if strings.HasPrefix(arg, "--basic-auth-file=") {
			args = append(args[:i], args[i+1:]...)
		}
	}
	files := []string{}
	if !s.DisableETCD {
		files = append(files, etcdNameFile(s.DataDir))
	}
	after(etcdReady, func() error {
		return staticpod.Run(s.ManifestsDir, staticpod.Args{
			Command:   "kube-apiserver",
			Args:      args,
			Image:     image,
			Dirs:      append(onlyExisting(ssldirs), filepath.Dir(auditLogFile)),
			CPUMillis: 250,
			Files:     files,
		})
	})
	return auth, http.NotFoundHandler(), err
}

// Scheduler starts the kube-scheduler static pod, once the apiserver is available.
func (s *StaticPodConfig) Scheduler(apiReady <-chan struct{}, args []string) error {
	image, err := s.Resolver.GetReference(images.KubeScheduler)
	if err != nil {
		return err
	}
	if err := images.Pull(s.ImagesDir, images.KubeScheduler, image); err != nil {
		return err
	}
	files := []string{}
	if !s.DisableETCD {
		files = append(files, etcdNameFile(s.DataDir))
	}
	return after(apiReady, func() error {
		return staticpod.Run(s.ManifestsDir, staticpod.Args{
			Command:     "kube-scheduler",
			Args:        args,
			Image:       image,
			HealthPort:  10251,
			HealthProto: "HTTP",
			CPUMillis:   100,
			Files:       files,
		})
	})
}

// onlyExisting filters out paths from the list that cannot be accessed
func onlyExisting(paths []string) []string {
	existing := []string{}
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}
	return existing
}

// after calls a function after a message is received from a channel.
func after(after <-chan struct{}, f func() error) error {
	go func() {
		<-after
		if err := f(); err != nil {
			logrus.Fatal(err)
		}
	}()
	return nil
}

// ControllerManager starts the kube-controller-manager static pod, once the apiserver is available.
func (s *StaticPodConfig) ControllerManager(apiReady <-chan struct{}, args []string) error {
	image, err := s.Resolver.GetReference(images.KubeControllerManager)
	if err != nil {
		return err
	}
	if err := images.Pull(s.ImagesDir, images.KubeControllerManager, image); err != nil {
		return err
	}
	if s.CloudProvider != nil {
		extraArgs := []string{
			"--cloud-provider=" + s.CloudProvider.Name,
			"--cloud-config=" + s.CloudProvider.Path,
		}
		args = append(extraArgs, args...)
	}
	files := []string{}
	if !s.DisableETCD {
		files = append(files, etcdNameFile(s.DataDir))
	}
	return after(apiReady, func() error {
		extraArgs := []string{
			"--flex-volume-plugin-dir=/var/lib/kubelet/volumeplugins",
			"--terminated-pod-gc-threshold=1000",
		}
		args = append(extraArgs, args...)
		return staticpod.Run(s.ManifestsDir, staticpod.Args{
			Command:     "kube-controller-manager",
			Args:        args,
			Image:       image,
			Dirs:        onlyExisting(ssldirs),
			HealthPort:  10252,
			HealthProto: "HTTP",
			CPUMillis:   200,
			Files:       files,
		})
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
func (s *StaticPodConfig) ETCD(args executor.ETCDConfig) error {
	image, err := s.Resolver.GetReference(images.ETCD)
	if err != nil {
		return err
	}
	if err := images.Pull(s.ImagesDir, images.ETCD, image); err != nil {
		return err
	}

	initial, err := json.Marshal(args.InitialOptions)
	if err != nil {
		return err
	}

	confFile, err := args.ToConfigFile()
	if err != nil {
		return err
	}

	spa := staticpod.Args{
		Annotations: map[string]string{
			"etcd.k3s.io/initial": string(initial),
		},
		Command: "etcd",
		Args: []string{
			"--config-file=" + confFile,
		},
		Image: image,
		Dirs:  []string{args.DataDir},
		Files: []string{
			args.ServerTrust.CertFile,
			args.ServerTrust.KeyFile,
			args.ServerTrust.TrustedCAFile,
			args.PeerTrust.CertFile,
			args.PeerTrust.KeyFile,
			args.PeerTrust.TrustedCAFile,
		},
		HealthPort:  2381,
		HealthPath:  "/health",
		HealthProto: "HTTP",
	}

	if s.CISMode {
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
		if spa.SecurityContext == nil {
			spa.SecurityContext = &v1.PodSecurityContext{}
		}
		spa.SecurityContext.RunAsUser = &uid
		spa.SecurityContext.RunAsGroup = &gid

		for _, p := range []string{args.DataDir, filepath.Dir(args.ServerTrust.CertFile)} {
			if err := chownr(p, int(uid), int(gid)); err != nil {
				return err
			}
		}
	}

	if cmds.AgentConfig.EnableSELinux {
		if spa.SecurityContext == nil {
			spa.SecurityContext = &v1.PodSecurityContext{}
		}
		if spa.SecurityContext.SELinuxOptions == nil {
			spa.SecurityContext.SELinuxOptions = &v1.SELinuxOptions{
				Type: "rke2_service_db_t",
			}
		}
	}

	return staticpod.Run(s.ManifestsDir, spa)
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

func etcdNameFile(dataDir string) string {
	return filepath.Join(dataDir, "server", "db", "etcd", "name")
}

func writeDefaultPolicyFile(policyFilePath string) error {
	auditPolicy := auditv1.Policy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Policy",
			APIVersion: "audit.k8s.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{},
		Rules: []auditv1.PolicyRule{
			{
				Level: "None",
			},
		},
	}
	bytes, err := yaml.Marshal(auditPolicy)
	if err != nil {
		return err
	}
	return writeIfNotExists(policyFilePath, bytes)
}

// writeIfNotExists writes content to a file at a given path, but only if the file does not already exist
func writeIfNotExists(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()
	_, err = file.Write(content)
	return err
}
