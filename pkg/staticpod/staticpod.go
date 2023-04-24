package staticpod

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/util/hash"
)

const (
	extraMountPrefix = "extra-mount"
)

type ProbeConf struct {
	InitialDelaySeconds int32
	TimeoutSeconds      int32
	FailureThreshold    int32
	PeriodSeconds       int32
}

type ProbeConfs struct {
	Liveness  ProbeConf
	Readiness ProbeConf
	Startup   ProbeConf
}

type Args struct {
	Command         string
	Args            []string
	Image           name.Reference
	Dirs            []string
	Files           []string
	ExcludeFiles    []string
	HealthExec      []string
	HealthPort      int32
	HealthProto     string
	HealthPath      string
	ReadyExec       []string
	ReadyPort       int32
	ReadyProto      string
	ReadyPath       string
	CPURequest      string
	CPULimit        string
	MemoryRequest   string
	MemoryLimit     string
	ExtraMounts     []string
	ExtraEnv        []string
	ProbeConfs      ProbeConfs
	SecurityContext *v1.PodSecurityContext
	Annotations     map[string]string
	Privileged      bool
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
	files, err := readFiles(args.Args, args.ExcludeFiles)
	if err != nil {
		return err
	}

	args.Files = append(args.Files, files...)
	pod, err := pod(args)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(dir, args.Command+".yaml")

	// Generate a stable UID based on the manifest path. This allows the kubelet to reconcile the pod's
	// containers even when the apiserver is unavailable. If the UID is not stable, the kubelet
	// will consider the manifest change as two separate add/remove operations, and may start the new pod
	// before terminating the old one. Cleanup of removed pods is disabled until all sources have synced,
	// so if the apiserver is down, the newly added pod may get stuck in a crash loop due to the old pod
	// still using its ports. See https://github.com/rancher/rke2/issues/3387
	hasher := md5.New()
	fmt.Fprint(hasher, manifestPath)
	pod.UID = types.UID(hex.EncodeToString(hasher.Sum(nil)[0:]))

	// Append a hash of the completed pod manifest to the container environment for later use when checking
	// to see if the pod has been updated. It's fine that setting this changes the actual hash; we
	// just need a stable values that we can compare between the file on disk and the running
	// container to see if the kubelet has reconciled yet.
	hash.DeepHashObject(hasher, pod)
	pod.Spec.Containers[0].Env = append(pod.Spec.Containers[0].Env, v1.EnvVar{Name: "POD_HASH", Value: hex.EncodeToString(hasher.Sum(nil)[0:])})

	b, err := yaml.Marshal(pod)
	if err != nil {
		return err
	}
	return writeFile(manifestPath, b)
}

func writeFile(dest string, content []byte) error {
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
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
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
					Name:    args.Command,
					Image:   args.Image.Name(),
					Command: []string{args.Command},
					Args:    args.Args,
					Env: []v1.EnvVar{
						{
							Name:  "FILE_HASH",
							Value: filehash,
						},
					},
					Resources: v1.ResourceRequirements{
						Requests: v1.ResourceList{},
						Limits:   v1.ResourceList{},
					},
					LivenessProbe:   livenessProbe(args),
					ReadinessProbe:  readinessProbe(args),
					StartupProbe:    startupProbe(args),
					ImagePullPolicy: v1.PullIfNotPresent,
					SecurityContext: &v1.SecurityContext{
						Privileged: &args.Privileged,
					},
				},
			},
			HostNetwork:       true,
			PriorityClassName: "system-cluster-critical",
			SecurityContext:   args.SecurityContext,
		},
	}

	if args.CPURequest != "" {
		if cpuRequest, err := resource.ParseQuantity(args.CPURequest); err != nil {
			logrus.Errorf("error parsing cpu request for static pod %s: %v", args.Command, err)
		} else {
			p.Spec.Containers[0].Resources.Requests[v1.ResourceCPU] = cpuRequest
		}
	}

	if args.CPULimit != "" {
		if cpuLimit, err := resource.ParseQuantity(args.CPULimit); err != nil {
			logrus.Errorf("error parsing cpu limit for static pod %s: %v", args.Command, err)
		} else {
			p.Spec.Containers[0].Resources.Limits[v1.ResourceCPU] = cpuLimit
		}
	}

	if args.MemoryRequest != "" {
		if memoryRequest, err := resource.ParseQuantity(args.MemoryRequest); err != nil {
			logrus.Errorf("error parsing memory request for static pod %s: %v", args.Command, err)
		} else {
			p.Spec.Containers[0].Resources.Requests[v1.ResourceMemory] = memoryRequest
		}
	}

	if args.MemoryLimit != "" {
		if memoryLimit, err := resource.ParseQuantity(args.MemoryLimit); err != nil {
			logrus.Errorf("error parsing memory limit for static pod %s: %v", args.Command, err)
		} else {
			p.Spec.Containers[0].Resources.Limits[v1.ResourceMemory] = memoryLimit
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

	addExtraMounts(p, args.ExtraMounts)
	addExtraEnv(p, args.ExtraEnv)

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

func addExtraMounts(p *v1.Pod, extraMounts []string) {
	var (
		sourceType = v1.HostPathDirectoryOrCreate
	)

	for i, rawMount := range extraMounts {
		mount := strings.Split(rawMount, ":")
		var ro bool
		switch len(mount) {
		case 2: // In the case of 2 elements, we expect this to be a traditional source:dest volume mount and should noop.
		case 3:
			switch strings.ToLower(mount[2]) {
			case "ro":
				ro = true
			case "rw":
				ro = false
			default:
				logrus.Errorf("unknown mount option: %s encountered in extra mount %s for pod %s", mount[2], rawMount, p.Name)
				continue
			}
		default:
			logrus.Errorf("mount for pod %s %s was not valid", p.Name, rawMount)
			continue
		}

		name := fmt.Sprintf("%s-%d", extraMountPrefix, i)
		p.Spec.Volumes = append(p.Spec.Volumes, v1.Volume{
			Name: name,
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: mount[0],
					Type: &sourceType,
				},
			},
		})
		p.Spec.Containers[0].VolumeMounts = append(p.Spec.Containers[0].VolumeMounts, v1.VolumeMount{
			Name:      name,
			ReadOnly:  ro,
			MountPath: mount[1],
		})
	}
}

func addExtraEnv(p *v1.Pod, extraEnv []string) {
	for _, rawEnv := range extraEnv {
		env := strings.SplitN(rawEnv, "=", 2)
		if len(env) != 2 {
			logrus.Errorf("environment variable for pod %s %s was not valid", p.Name, rawEnv)
			continue
		}
		p.Spec.Containers[0].Env = append(p.Spec.Containers[0].Env, v1.EnvVar{
			Name:  env[0],
			Value: env[1],
		})
	}
}

func readFiles(args, excludeFiles []string) ([]string, error) {
	files := map[string]bool{}
	excludes := map[string]bool{}

	for _, file := range excludeFiles {
		excludes[file] = true
	}

	for _, arg := range args {
		parts := strings.SplitN(arg, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[1], string(os.PathSeparator)) {
			if stat, err := os.Stat(parts[1]); err == nil && !stat.IsDir() && !excludes[parts[1]] {
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

// livenessProbe returns a Probe, using the Health values from the provided pod args,
// and the appropriate thresholds for liveness probing.
func livenessProbe(args Args) *v1.Probe {
	return createProbe(args.HealthExec, args.HealthPath, args.HealthProto, args.HealthPort, args.ProbeConfs.Liveness)
}

// readinessProbe returns a Probe, using the Ready values from the provided pod args,
// and the appropriate thresholds for readiness probing.
func readinessProbe(args Args) *v1.Probe {
	return createProbe(args.ReadyExec, args.ReadyPath, args.ReadyProto, args.ReadyPort, args.ProbeConfs.Readiness)
}

// startupProbe returns a Probe, using the Health values from the provided pod args,
// and the appropriate thresholds for startup probing.
func startupProbe(args Args) *v1.Probe {
	return createProbe(args.HealthExec, args.HealthPath, args.HealthProto, args.HealthPort, args.ProbeConfs.Startup)
}

// createProbe creates a Probe using the provided configuration.
// If command is set, an ExecAction Probe is returned.
// If command is empty but port is set, a HTTPGetAction Probe is returned.
// If neither is set, no Probe is returned.
func createProbe(command []string, path, scheme string, port int32, conf ProbeConf) *v1.Probe {
	probe := &v1.Probe{
		InitialDelaySeconds: conf.InitialDelaySeconds,
		TimeoutSeconds:      conf.TimeoutSeconds,
		FailureThreshold:    conf.FailureThreshold,
		PeriodSeconds:       conf.PeriodSeconds,
	}
	if len(command) != 0 {
		probe.Exec = &v1.ExecAction{
			Command: command,
		}
		if probe.PeriodSeconds < 5 {
			probe.PeriodSeconds = 5
		}
		return probe
	} else if port != 0 {
		probe.HTTPGet = &v1.HTTPGetAction{
			Path:   path,
			Host:   "localhost",
			Scheme: v1.URIScheme(scheme),
			Port: intstr.IntOrString{
				IntVal: port,
			},
		}
		if probe.HTTPGet.Scheme == "" {
			probe.HTTPGet.Scheme = v1.URISchemeHTTPS
		}
		if probe.HTTPGet.Path == "" {
			probe.HTTPGet.Path = "/healthz"
		}
		return probe
	}
	return nil
}
