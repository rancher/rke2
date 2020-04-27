package podexecutor

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"k8s.io/apiserver/plugin/pkg/authenticator/password/passwordfile"
	"k8s.io/apiserver/plugin/pkg/authenticator/request/basicauth"

	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/wrangler/pkg/yaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apiserver/pkg/authentication/authenticator"
)

var (
	ssldirs = []string{
		"/etc/ssl/certs",
		"/etc/ca-certificates",
		"/usr/local/share/ca-certificates",
		"/usr/share/ca-certificates",
	}
)

type StaticPod struct {
	Manifests string
	Images    images.Images
}

type PodArgs struct {
	Command    string
	Args       []string
	Image      string
	Dirs       []string
	Files      []string
	HealthPort int32
	CPUMillis  int64
}

func (s *StaticPod) Kubelet(args []string) error {
	go func() {
		for {
			fmt.Println("!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!! START", args)
			cmd := exec.Command("kubelet", args...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			cmd.Run()
		}
	}()

	return nil
}

func (s *StaticPod) KubeProxy(args []string) error {
	panic("kube-proxy unsupported")
}

func (s *StaticPod) APIServer(ctx context.Context, args []string) (authenticator.Request, http.Handler, bool, error) {
	for i, arg := range args {
		if strings.HasPrefix(arg, "--advertise-port=") {
			args = append(args[:i], args[i+1:]...)
			break
		}
	}

	err := RunPod(s.Manifests, PodArgs{
		Command:    "kube-apiserver",
		Args:       args,
		Image:      s.Images.KubeAPIServer,
		Dirs:       ssldirs,
		HealthPort: 6443,
		CPUMillis:  250,
	})
	if err != nil {
		return nil, nil, false, err
	}
	auth, err := auth(args)
	return auth, http.NotFoundHandler(), false, err
}

func auth(args []string) (authenticator.Request, error) {
	for _, arg := range args {
		if !strings.HasPrefix(arg, "--basic-auth-file=") {
			continue
		}
		file := strings.SplitN(arg, "=", 2)[1]
		basicAuthenticator, err := passwordfile.NewCSV(file)
		if err != nil {
			return nil, err
		}

		return basicauth.New(basicAuthenticator), nil
	}

	return nil, nil
}

func (s *StaticPod) Scheduler(args []string) error {
	if true {
		return nil
	}
	return RunPod(s.Manifests, PodArgs{
		Command:    "kube-scheduler",
		Args:       args,
		Image:      s.Images.KubeScheduler,
		HealthPort: 10259,
		CPUMillis:  100,
	})
}

func (s *StaticPod) ControllerManager(args []string) error {
	if true {
		return nil
	}
	return RunPod(s.Manifests, PodArgs{
		Command: "kube-controller-manager",
		Args: append(args,
			"/usr/libexec/kubernetes/kubelet-plugins/volume/exec"),
		Image:      s.Images.KubeControllManager,
		HealthPort: 10257,
		CPUMillis:  200,
	})
}

func RunPod(dir string, args PodArgs) error {
	seen := map[string]bool{}
	for _, arg := range args.Args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[1], "/") {
			if stat, err := os.Stat(parts[1]); err == nil && !stat.IsDir() {
				if seen[parts[1]] {
					continue
				}
				seen[parts[1]] = true
				args.Files = append(args.Files, parts[1])
			}
		}
	}

	pod := pod(args)
	bytes, err := yaml.Export(pod)
	if err != nil {
		return err
	}

	return writeFile(dir, args.Command, bytes)
}

func writeFile(dir, name string, content []byte) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	dest := filepath.Join(dir, name+".yaml")
	tmp := filepath.Join(dir, name+".tmp")
	if err := ioutil.WriteFile(tmp, content, 0777); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func pod(args PodArgs) *v1.Pod {
	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.Command,
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": args.Command,
				"tier":      "control-plane",
			},
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Command:         append([]string{args.Command}, args.Args...),
					Image:           args.Image,
					ImagePullPolicy: v1.PullIfNotPresent,
					LivenessProbe: &v1.Probe{
						Handler: v1.Handler{
							HTTPGet: &v1.HTTPGetAction{
								Path: "/healthz",
								Port: intstr.IntOrString{
									IntVal: args.HealthPort,
								},
								Host:   "127.0.0.1",
								Scheme: "HTTPS",
							},
						},
						InitialDelaySeconds: 15,
						TimeoutSeconds:      15,
						FailureThreshold:    8,
					},
					Name: args.Command,
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{
							v1.ResourceCPU: *resource.NewMilliQuantity(args.CPUMillis, resource.DecimalSI),
						},
					},
				},
			},
			HostNetwork:       true,
			PriorityClassName: "system-cluster-critical",
		},
	}

	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if !strings.Contains(strings.ToLower(parts[0]), "proxy") {
			continue
		}
		if len(parts) == 1 {
			p.Spec.Containers[0].Env = append(p.Spec.Containers[0].Env, v1.EnvVar{
				Name: parts[0],
			})
		} else {
			p.Spec.Containers[0].Env = append(p.Spec.Containers[0].Env, v1.EnvVar{
				Name:  parts[0],
				Value: parts[1],
			})
		}
	}

	addVolumes(p, args.Dirs, true)
	addVolumes(p, args.Files, false)

	return p
}

func addVolumes(p *v1.Pod, src []string, dir bool) {
	var (
		prefix     = "dir"
		sourceType = v1.HostPathDirectoryOrCreate
	)
	if !dir {
		prefix = "file"
		sourceType = v1.HostPathFile
	}

	for i, src := range src {
		name := fmt.Sprintf("%s%d", prefix, i)
		p.Spec.Volumes = append(p.Spec.Volumes, v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: src,
					Type: &sourceType,
				},
			},
		})
		p.Spec.Containers[0].VolumeMounts = append(p.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      name,
			ReadOnly:  true,
			MountPath: src,
		})
	}

}
