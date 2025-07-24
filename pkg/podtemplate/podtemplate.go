package podtemplate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
)

type typeVolume string

const (
	extraMountPrefix            = "extra-mount"
	socket           typeVolume = "socket"
	dir              typeVolume = "dir"
	file             typeVolume = "file"
)

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

func Pod(spec *Spec) (*v1.Pod, error) {
	if spec == nil {
		return nil, nil
	}

	filehash, err := hashFiles(spec.Files)
	if err != nil {
		return nil, err
	}

	p := &v1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      spec.Command,
			Namespace: "kube-system",
			Labels: map[string]string{
				"component": spec.Command,
				"tier":      "control-plane",
			},
			Annotations: spec.Annotations,
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{
				{
					Name:    spec.Command,
					Image:   spec.Image.Name(),
					Command: []string{spec.Command},
					Args:    spec.Args,
					Ports:   spec.Ports,
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
					LivenessProbe:   spec.livenessProbe(),
					ReadinessProbe:  spec.readinessProbe(),
					StartupProbe:    spec.startupProbe(),
					ImagePullPolicy: v1.PullIfNotPresent,
					SecurityContext: &v1.SecurityContext{
						Privileged: &spec.Privileged,
					},
				},
			},
			HostNetwork:       spec.HostNetwork,
			PriorityClassName: "system-cluster-critical",
			SecurityContext:   spec.SecurityContext,
		},
	}

	if spec.CPURequest != "" {
		if cpuRequest, err := resource.ParseQuantity(spec.CPURequest); err != nil {
			logrus.Errorf("error parsing cpu request for static pod %s: %v", spec.Command, err)
		} else {
			p.Spec.Containers[0].Resources.Requests[v1.ResourceCPU] = cpuRequest
		}
	}

	if spec.CPULimit != "" {
		if cpuLimit, err := resource.ParseQuantity(spec.CPULimit); err != nil {
			logrus.Errorf("error parsing cpu limit for static pod %s: %v", spec.Command, err)
		} else {
			p.Spec.Containers[0].Resources.Limits[v1.ResourceCPU] = cpuLimit
		}
	}

	if spec.MemoryRequest != "" {
		if memoryRequest, err := resource.ParseQuantity(spec.MemoryRequest); err != nil {
			logrus.Errorf("error parsing memory request for static pod %s: %v", spec.Command, err)
		} else {
			p.Spec.Containers[0].Resources.Requests[v1.ResourceMemory] = memoryRequest
		}
	}

	if spec.MemoryLimit != "" {
		if memoryLimit, err := resource.ParseQuantity(spec.MemoryLimit); err != nil {
			logrus.Errorf("error parsing memory limit for static pod %s: %v", spec.Command, err)
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

	addVolumes(p, spec.Sockets, socket)
	addVolumes(p, spec.Dirs, dir)
	addVolumes(p, spec.Files, file)

	addExtraMounts(p, spec.ExtraMounts)
	addExtraEnv(p, spec.ExtraEnv)

	return p, nil
}

func addVolumes(p *v1.Pod, src []string, volume typeVolume) {
	var (
		prefix     string
		sourceType v1.HostPathType
		readOnly   bool
	)

	prefix = string(volume)
	switch volume {
	case dir:
		sourceType = v1.HostPathDirectoryOrCreate
		readOnly = false
	case socket:
		sourceType = v1.HostPathSocket
		readOnly = false
	default:
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
	for i, rawMount := range extraMounts {
		var sourceType v1.HostPathType
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
				logrus.Errorf("Unknown mount option: %s encountered in extra mount %s for pod %s", mount[2], rawMount, p.Name)
				continue
			}
		case 4:
			sourceType = v1.HostPathType(mount[3])
		default:
			logrus.Errorf("Extra mount for pod %s %s was not valid", p.Name, rawMount)
			continue
		}

		// If the source type was not specified, try to auto-detect.
		// Paths that cannot be stat-ed are handled as DirectoryOrCreate.
		// Only sockets, directories, and files are supported for auto-detection.
		if sourceType == v1.HostPathUnset {
			if info, err := os.Stat(mount[0]); err != nil {
				if !os.IsNotExist(err) {
					logrus.Warnf("Failed to stat mount for pod %s %s: %v", p.Name, mount[0], err)
				}
				sourceType = v1.HostPathDirectoryOrCreate
			} else {
				switch {
				case info.Mode().Type() == fs.ModeSocket:
					sourceType = v1.HostPathSocket
				case info.IsDir():
					sourceType = v1.HostPathDirectory
				default:
					sourceType = v1.HostPathFile
				}
			}
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

// ReadFiles takes in the arguments passed to the static pod and returns a list of all files
// embedded in those arguments to be included in the pod manifest as volumes.
// excludeFiles are not included in the returned list.
func ReadFiles(args, excludeFiles []string) ([]string, error) {
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
func (s *Spec) livenessProbe() *v1.Probe {
	var host string
	if s.HostNetwork {
		host = "localhost"
	}
	return createProbe(s.HealthExec, s.HealthScheme, host, s.HealthPath, s.HealthPort, s.ProbeConfs.Liveness)
}

// readinessProbe returns a Probe, using the Ready values from the provided pod args,
// and the appropriate thresholds for readiness probing.
func (s *Spec) readinessProbe() *v1.Probe {
	var host string
	if s.HostNetwork {
		host = "localhost"
	}
	return createProbe(s.ReadyExec, s.ReadyScheme, host, s.ReadyPath, s.ReadyPort, s.ProbeConfs.Readiness)
}

// startupProbe returns a Probe, using the Startup values from the provided pod args,
// and the appropriate thresholds for startup probing.
func (s *Spec) startupProbe() *v1.Probe {
	var host string
	if s.HostNetwork {
		host = "localhost"
	}
	return createProbe(s.StartupExec, s.StartupScheme, host, s.StartupPath, s.StartupPort, s.ProbeConfs.Startup)
}

// createProbe creates a Probe using the provided configuration.
// If command is set, an ExecAction Probe is returned.
// If command is empty but port is set, a HTTPGetAction Probe is returned.
// If neither is set, no Probe is returned.
func createProbe(command []string, scheme, host, path string, port int32, conf ProbeConf) *v1.Probe {
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
			Host:   host,
			Scheme: v1.URIScheme(scheme),
			Port: intstr.IntOrString{
				IntVal: port,
			},
		}
		if probe.HTTPGet.Scheme == "" {
			probe.HTTPGet.Scheme = v1.URISchemeHTTPS
		}
		if probe.HTTPGet.Path == "" {
			probe.HTTPGet.Path = "/livez"
		}
		return probe
	}
	return nil
}
