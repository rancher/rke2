package podexecutor

import (
	"context"
	"encoding/json"
	"io/ioutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/executor"
	"github.com/rancher/rke2/pkg/auth"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/rke2/pkg/staticpod"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	auditv1 "k8s.io/apiserver/pkg/apis/audit/v1"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"sigs.k8s.io/yaml"
)

var (
	ssldirs = []string{
		"/etc/ssl/certs",
		"/etc/ca-certificates",
		"/usr/local/share/ca-certificates",
		"/usr/share/ca-certificates",
	}
)

type StaticPodConfig struct {
	ManifestsDir    string
	ImagesDir       string
	Images          images.Images
	CloudProvider   *CloudProviderConfig
	CISMode         bool
	DataDir         string
	AuditPolicyFile string
}

type CloudProviderConfig struct {
	Name string
	Path string
}

// Kubelet starts the kubelet in a subprocess with watching goroutine.
func (s *StaticPodConfig) Kubelet(args []string) error {
	if s.CloudProvider != nil {
		args = append(args,
			"--cloud-provider="+s.CloudProvider.Name,
			"--cloud-config="+s.CloudProvider.Path)
	}
	go func() {
		for {
			cmd := exec.Command("kubelet", args...)
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

// KubeProxy panics if used. KubeProxy is not supported in RKE2.
func (s *StaticPodConfig) KubeProxy(args []string) error {
	panic("kube-proxy unsupported")
}

// APIServer sets up the apiserver static pod once etcd is available, returning the authenticator and request handler.
func (s *StaticPodConfig) APIServer(ctx context.Context, etcdReady <-chan struct{}, args []string) (authenticator.Request, http.Handler, error) {
	auditLogFile := filepath.Join(s.DataDir, "server/logs/audit.log")
	if s.CloudProvider != nil {
		args = append(args,
			"--cloud-provider="+s.CloudProvider.Name,
			"--cloud-config="+s.CloudProvider.Path)
	}
	if err := images.Pull(s.ImagesDir, "kube-apiserver", s.Images.KubeAPIServer); err != nil {
		return nil, nil, err
	}

	if s.CISMode {
		args = append(args,
			"--audit-policy-file="+s.AuditPolicyFile,
			"--audit-log-path="+auditLogFile,
			"--audit-log-maxage=30",
			"--audit-log-maxbackup=10",
			"--audit-log-maxsize=100",
		)
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
	after(etcdReady, func() error {
		return staticpod.Run(s.ManifestsDir, staticpod.Args{
			Command:   "kube-apiserver",
			Args:      args,
			Image:     s.Images.KubeAPIServer,
			Dirs:      append(ssldirs, filepath.Dir(auditLogFile)),
			CPUMillis: 250,
			Files:     []string{etcdNameFile(s.DataDir)},
		})
	})
	return auth, http.NotFoundHandler(), err
}

// Scheduler starts the kube-scheduler static pod, once the apiserver is available.
func (s *StaticPodConfig) Scheduler(apiReady <-chan struct{}, args []string) error {
	if err := images.Pull(s.ImagesDir, "kube-scheduler", s.Images.KubeScheduler); err != nil {
		return err
	}
	return after(apiReady, func() error {
		return staticpod.Run(s.ManifestsDir, staticpod.Args{
			Command:     "kube-scheduler",
			Args:        args,
			Image:       s.Images.KubeScheduler,
			HealthPort:  10251,
			HealthProto: "HTTP",
			CPUMillis:   100,
			Files:       []string{etcdNameFile(s.DataDir)},
		})
	})
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
	if s.CloudProvider != nil {
		args = append(args,
			"--cloud-provider="+s.CloudProvider.Name,
			"--cloud-config="+s.CloudProvider.Path)
	}

	if err := images.Pull(s.ImagesDir, "kube-controller-manager", s.Images.KubeControllManager); err != nil {
		return err
	}
	return after(apiReady, func() error {
		return staticpod.Run(s.ManifestsDir, staticpod.Args{
			Command: "kube-controller-manager",
			Args: append(args,
				"--flex-volume-plugin-dir=/usr/libexec/kubernetes/kubelet-plugins/volume/exec",
				"--terminated-pod-gc-threshold=1000",
			),
			Image:       s.Images.KubeControllManager,
			HealthPort:  10252,
			HealthProto: "HTTP",
			CPUMillis:   200,
			Files:       []string{etcdNameFile(s.DataDir)},
		})
	})
}

// CurrentETCDOptions retrieves the etcd configuration from the static pod definition at etcd.yaml
// in the manifests directory, if it exists.
func (s *StaticPodConfig) CurrentETCDOptions() (opts executor.InitialOptions, err error) {
	bytes, err := ioutil.ReadFile(filepath.Join(s.ManifestsDir, "etcd.yaml"))
	if os.IsNotExist(err) {
		return
	}

	pod := &v1.Pod{}
	if err := yaml.Unmarshal(bytes, pod); err != nil {
		return opts, err
	}

	v, ok := pod.Annotations["etcd.k3s.io/initial"]
	if ok {
		return opts, json.NewDecoder(strings.NewReader(v)).Decode(&opts)
	}

	return
}

// ETCD starts the etcd static pod.
func (s *StaticPodConfig) ETCD(args executor.ETCDConfig) error {
	if err := images.Pull(s.ImagesDir, "etcd", s.Images.ETCD); err != nil {
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
		Image: s.Images.ETCD,
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
			auditv1.PolicyRule{
				Level: "Metadata",
				Resources: []auditv1.GroupResources{
					auditv1.GroupResources{
						Group:     "",
						Resources: []string{"pods"},
					},
				},
			},
		},
	}
	bytes, err := yaml.Marshal(auditPolicy)
	if err != nil {
		return err
	}
	return writeArgFile(policyFilePath, bytes)
}

func writeArgFile(path string, content []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	if _, err := os.Stat(path); err == nil {
		return nil
	} else {
		if !os.IsNotExist(err) {
			return err
		}
	}
	return ioutil.WriteFile(path, content, 0600)
}
