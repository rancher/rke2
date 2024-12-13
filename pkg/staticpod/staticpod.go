package staticpod

import (
	"bytes"
	"crypto/md5"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/util/hash"
	"sigs.k8s.io/yaml"
)

type typeVolume string

const (
	extraMountPrefix            = "extra-mount"
	socket           typeVolume = "socket"
	dir              typeVolume = "dir"
	file             typeVolume = "file"
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
	Sockets         []string
	CISMode         bool // CIS requires that the manifest be saved with 600 permissions
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

// Remove cleans up the static pod manifest for the given command from the specified directory.
// It does not actually stop or remove the static pod from the container runtime.
func Remove(dir, command string) error {
	manifestPath := filepath.Join(dir, command+".yaml")
	if err := os.Remove(manifestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Wrapf(err, "failed to remove %s static pod manifest", command)
	}
	logrus.Infof("Removed %s static pod manifest", command)
	return nil
}

// Run writes a static pod manifest for the given command into the specified directory.
// Note that it does not actually run the command; the kubelet is responsible for picking up
// the manifest and creating container to run it.
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

	// TODO Check to make sure we aren't double mounting directories and the files in those directories

	args.Files = append(args.Files, files...)
	pod, err := pod(args)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(dir, args.Command+".yaml")

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
	if args.CISMode {
		return writeFile(manifestPath, b, 0600)
	}
	return writeFile(manifestPath, b, 0644)
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

	addVolumes(p, args.Sockets, socket)
	addVolumes(p, args.Dirs, dir)
	addVolumes(p, args.Files, file)

	addExtraMounts(p, args.ExtraMounts)
	addExtraEnv(p, args.ExtraEnv)

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

// readFiles takes in the arguments passed to the static pod and returns a list of all files
// embedded in those arguments to be included in the pod manifest as volumes.
// excludeFiles are not included in the returned list.
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
