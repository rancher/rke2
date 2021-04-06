package staticpod

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/wrangler/pkg/yaml"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
)

type Args struct {
	Command         string
	Args            []string
	Image           name.Reference
	Dirs            []string
	Files           []string
	HealthPort      int32
	HealthProto     string
	HealthPath      string
	CPUMillis       int64
	SecurityContext *v1.PodSecurityContext
	Annotations     map[string]string
}

func Run(dir string, args Args) error {
	if cmds.AgentConfig.EnableSELinux {
		if args.SecurityContext == nil {
			args.SecurityContext = &v1.PodSecurityContext{}
		}
		if args.SecurityContext.SELinuxOptions == nil {
			args.SecurityContext.SELinuxOptions = &v1.SELinuxOptions{
				Type: "rke2_service_t",
			}
		}
	}
	files, err := readFiles(args.Args)
	if err != nil {
		return err
	}

	args.Files = append(args.Files, files...)
	pod, err := pod(args)
	if err != nil {
		return err
	}

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

	existing, err := ioutil.ReadFile(dest)
	if err == nil && bytes.Equal(existing, content) {
		return nil
	}

	tmp := filepath.Join(dir, name+".tmp")
	if err := ioutil.WriteFile(tmp, content, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func hashFiles(files []string) (string, error) {
	h := sha256.New()
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			return "", err
		}
		_, err = io.Copy(h, f)
		f.Close()
		if err != nil {
			return "", err
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func pod(args Args) (*v1.Pod, error) {
	filehash, err := hashFiles(args.Files)
	if err != nil {
		return nil, err
	}

	p := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      args.Command,
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": args.Command,
				"tier":      "control-plane",
			},
			Annotations: args.Annotations,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Command:         append([]string{args.Command}, args.Args...),
					Image:           args.Image.Name(),
					ImagePullPolicy: v1.PullIfNotPresent,
					Env: []v1.EnvVar{
						{
							Name:  "FILE_HASH",
							Value: filehash,
						},
					},
					Name: args.Command,
				},
			},
			HostNetwork:       true,
			PriorityClassName: "system-cluster-critical",
			SecurityContext:   args.SecurityContext,
		},
	}

	if args.CPUMillis > 0 {
		p.Spec.Containers[0].Resources = v1.ResourceRequirements{
			Requests: v1.ResourceList{
				v1.ResourceCPU: *resource.NewMilliQuantity(args.CPUMillis, resource.DecimalSI),
			},
		}
	}

	if args.HealthPort != 0 {
		scheme := args.HealthProto
		if scheme == "" {
			scheme = "HTTPS"
		}
		path := args.HealthPath
		if path == "" {
			path = "/healthz"
		}
		p.Spec.Containers[0].LivenessProbe = &v1.Probe{
			Handler: v1.Handler{
				HTTPGet: &v1.HTTPGetAction{
					Path: path,
					Port: intstr.IntOrString{
						IntVal: args.HealthPort,
					},
					Host:   "127.0.0.1",
					Scheme: v1.URIScheme(scheme),
				},
			},
			InitialDelaySeconds: 15,
			TimeoutSeconds:      15,
			FailureThreshold:    8,
		}
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

	return p, nil
}

func addVolumes(p *v1.Pod, src []string, dir bool) {
	var (
		prefix     = "dir"
		sourceType = v1.HostPathDirectoryOrCreate
		readOnly   = false
	)
	if !dir {
		prefix = "file"
		sourceType = v1.HostPathFile
		readOnly = true
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
			ReadOnly:  readOnly,
			MountPath: src,
		})
	}
}

func readFiles(args []string) ([]string, error) {
	files := map[string]bool{}

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[1], "/") {
			if stat, err := os.Stat(parts[1]); err == nil && !stat.IsDir() && !strings.Contains(parts[1], "audit.log") {
				files[parts[1]] = true

				if parts[0] == "--kubeconfig" {
					certs, err := kubeconfigFiles(parts[1])
					if err != nil {
						return nil, err
					}
					for _, cert := range certs {
						files[cert] = true
					}
				}
			}
		}
	}

	var result []string
	for k := range files {
		result = append(result, k)
	}
	sort.Strings(result)
	return result, nil
}

func kubeconfigFiles(kubeconfig string) ([]string, error) {
	var result []string

	kc, err := clientcmd.LoadFromFile(kubeconfig)
	if err != nil {
		return nil, err
	}

	for _, cluster := range kc.Clusters {
		if cluster.CertificateAuthority != "" {
			result = append(result, cluster.CertificateAuthority)
		}
	}

	for _, auth := range kc.AuthInfos {
		if auth.ClientKey != "" {
			result = append(result, auth.ClientKey)
		}
		if auth.ClientCertificate != "" {
			result = append(result, auth.ClientCertificate)
		}
	}

	return result, nil
}
