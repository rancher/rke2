package hosted

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"io/ioutil"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/executor"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/rancher/rke2/pkg/podtemplate"
	yaml2 "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/yaml"
)

// pathGroup is used to group Volume mounts by path prefix
type pathGroup struct {
	base   string
	prefix string
	paths  sets.Set[string]
}

// pod is essentially a wrapper around the normal static pod manifest,
// but with HostPath mounts converted to Secret volumes.
func (h *HostedConfig) pod(spec *podtemplate.Spec) (*corev1.Pod, error) {
	if spec == nil {
		return nil, nil
	}

	files, err := podtemplate.ReadFiles(spec.Args, spec.ExcludeFiles)
	if err != nil {
		return nil, err
	}

	spec.Files = append(spec.Files, files...)
	pod, err := podtemplate.Pod(spec)
	if err != nil {
		return nil, err
	}

	// Fix up name/namespace/labels and disable unnecessary things
	pod.Labels[clusterNameLabel] = h.Name
	pod.Name = fmt.Sprintf("%s-%s", h.Name, pod.Name)
	pod.Namespace = h.namespace
	pod.Spec.AutomountServiceAccountToken = ptr.To(false)

	// Convert individual HostPath mounts to Secret volumes,
	// grouped by top-level directory.
	volumesToRemove := sets.Set[string]{}
	pathGroups := []pathGroup{}

	for _, dir := range secretDirs {
		prefix := filepath.Join(h.DataDir, "server", dir) + string(filepath.Separator)
		pathGroups = append(pathGroups, pathGroup{base: dir, prefix: prefix, paths: sets.Set[string]{}})
	}

	// Remove VolumeMounts from Containers for anything that will be mounted from a Secret
	for i := range pod.Spec.Containers {
		pod.Spec.Containers[i].VolumeMounts = slices.DeleteFunc(pod.Spec.Containers[i].VolumeMounts, func(vm corev1.VolumeMount) bool {
			for _, pg := range pathGroups {
				if path, ok := strings.CutPrefix(vm.MountPath, pg.prefix); ok {
					pg.paths.Insert(path)
					volumesToRemove.Insert(vm.Name)
					return true
				}
			}
			return false
		})
		for _, pg := range pathGroups {
			if pg.paths.Len() > 0 {
				pod.Spec.Containers[i].VolumeMounts = append(pod.Spec.Containers[i].VolumeMounts, corev1.VolumeMount{
					MountPath: pg.prefix,
					Name:      "secret-server-" + pg.base,
					ReadOnly:  true,
				})
			}
		}
	}

	// Remove Volumes for removed VolumeMounts
	pod.Spec.Volumes = slices.DeleteFunc(pod.Spec.Volumes, func(v corev1.Volume) bool {
		return volumesToRemove.Has(v.Name)
	})

	// Add Volumes for Secrets
	for _, pg := range pathGroups {
		if pg.paths.Len() > 0 {
			volume := corev1.Volume{
				Name: "secret-server-" + pg.base,
				VolumeSource: corev1.VolumeSource{
					Secret: &corev1.SecretVolumeSource{
						SecretName: fmt.Sprintf("%s-server-%s", h.Name, pg.base),
					},
				},
			}
			paths := pg.paths.UnsortedList()
			slices.Sort(paths)
			for _, path := range paths {
				item := corev1.KeyToPath{Path: path}
				path = strings.ReplaceAll(path, string(filepath.Separator), "_")
				item.Key = path
				volume.Secret.Items = append(volume.Secret.Items, item)
			}
			pod.Spec.Volumes = append(pod.Spec.Volumes, volume)
		}
	}

	// Add anti-affinity on node hostname
	pod.Spec.Affinity = &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				corev1.WeightedPodAffinityTerm{
					Weight: 100,
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: corev1.LabelHostname,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: pod.Labels,
						},
					},
				},
			},
		},
	}

	return pod, nil
}

// applyApiserverService creates a Service for the apiserver
func (h *HostedConfig) applyAPIServerService(ctx context.Context) error {
	serviceType := corev1.ServiceTypeClusterIP
	if typeEnv := os.Getenv(version.ProgramUpper + "_CLUSTER_SERVICETYPE"); typeEnv != "" {
		serviceType = corev1.ServiceType(typeEnv)
	}

	service := &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name + "-kube-apiserver",
			Namespace: h.namespace,
			Labels: map[string]string{
				clusterNameLabel: h.Name,
				"component":      "kube-apiserver",
				"tier":           "control-plane",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: serviceType,
			Selector: map[string]string{
				clusterNameLabel: h.Name,
				"component":      "kube-apiserver",
				"tier":           "control-plane",
			},
			Ports: []corev1.ServicePort{
				{Name: "https", Port: int32(cmds.ServerConfig.APIServerPort), TargetPort: intstr.FromInt(cmds.ServerConfig.APIServerPort)},
				{Name: "supervisor", Port: int32(cmds.ServerConfig.SupervisorPort), TargetPort: intstr.FromInt(cmds.ServerConfig.SupervisorPort)},
			},
		},
	}

	return h.apply.WithSetID(h.Name + "-kube-apiserver-service").ApplyObjects(service)
}

// applyClusterSecrets creates secrets containing the contents of the
// server directories. This content is mounted from the  host by static pods,
// but is packaged into a Secret for use by the Deployment pods.
func (h *HostedConfig) applyClusterSecrets(ctx context.Context, dirs ...string) error {
	objs := []runtime.Object{}
	for _, dir := range dirs {
		data, err := h.bytesFromDir(filepath.Join(h.DataDir, "server", dir))
		if err != nil {
			return err
		}
		objs = append(objs, &corev1.Secret{
			TypeMeta: metav1.TypeMeta{
				APIVersion: "v1",
				Kind:       "Secret",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      h.Name + "-server-" + dir,
				Namespace: h.namespace,
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		})
	}

	return h.apply.WithSetID(h.Name + "-bootstrap-secrets").ApplyObjects(objs...)
}

// extractClusterSecrets extracts the secrets out to the data dir, so that
// bootstrap reconcile can update them as needed without recreating or reextracting
// them all from the datastore. This allows the supervisor pod to avoid needing persistent
// storage for cluster bootstrap data.
func (h *HostedConfig) extractClusterSecrets(ctx context.Context, dirs ...string) error {
	secrets := h.client.CoreV1().Secrets(h.namespace)
	for _, dir := range dirs {
		secret, err := secrets.Get(ctx, h.Name+"-server-"+dir, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return err
		}
		if err := h.dirFromBytes(filepath.Join(h.DataDir, "server", dir), secret.Data); err != nil {
			return err
		}
	}
	return nil
}

func (h *HostedConfig) statefulSetWithService(args *podtemplate.Spec) (*appsv1.StatefulSet, *corev1.Service, error) {
	if args == nil {
		return nil, nil, nil
	}

	pod, err := h.pod(args)
	return statefulSetForPod(pod), serviceForPod(pod), err
}

func statefulSetForPod(pod *corev1.Pod) *appsv1.StatefulSet {
	if pod == nil {
		return nil
	}

	replicas := 1
	maxUnavailable := 1

	if replicasEnv := os.Getenv(version.ProgramUpper + "_STATEFULSET_REPLICAS"); replicasEnv != "" {
		if r, err := strconv.Atoi(replicasEnv); err == nil && r >= 0 {
			replicas = r
		}
	}

	if replicas > 2 {
		maxUnavailable = replicas - 2
	}

	for i, c := range pod.Spec.Containers {
		c.Env = append(c.Env, corev1.EnvVar{
			Name: "POD_NAME",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.labels['statefulset.kubernetes.io/pod-name']",
				},
			},
		})
		pod.Spec.Containers[i] = c
	}

	// MaxUnavailable defaults to 1; only set if non-default
	// to avoid unnecessary patching
	var rollingUpdate *appsv1.RollingUpdateStatefulSetStrategy
	if maxUnavailable != 1 {
		rollingUpdate = &appsv1.RollingUpdateStatefulSetStrategy{
			MaxUnavailable: ptr.To(intstr.FromInt(maxUnavailable)),
		}
	}

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:             ptr.To(int32(replicas)),
			RevisionHistoryLimit: ptr.To(int32(0)),
			ServiceName:          pod.Name,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type:          appsv1.RollingUpdateStatefulSetStrategyType,
				RollingUpdate: rollingUpdate,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: pod.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: pod.Labels,
				},
				Spec: pod.Spec,
			},
		},
	}
}

func serviceForPod(pod *corev1.Pod) *corev1.Service {
	if pod == nil {
		return nil
	}

	ports := []corev1.ServicePort{}
	for _, port := range pod.Spec.Containers[0].Ports {
		ports = append(ports, corev1.ServicePort{Name: port.Name, Port: port.ContainerPort, TargetPort: intstr.FromInt32(port.ContainerPort)})
	}

	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
		},
		Spec: corev1.ServiceSpec{
			Type:      corev1.ServiceTypeClusterIP,
			ClusterIP: "None",
			Selector:  pod.Labels,
			Ports:     ports,
		},
	}
}

// deployment returns a Deployment for the provided podtemplate.Spec.
func (h *HostedConfig) deployment(spec *podtemplate.Spec) (*appsv1.Deployment, error) {
	if spec == nil {
		return nil, nil
	}

	pod, err := h.pod(spec)
	return deploymentForPod(pod), err
}

// deploymentForPod creates a Deployment with Name, LabelSelector, and Pod from the Pod
func deploymentForPod(pod *corev1.Pod) *appsv1.Deployment {
	if pod == nil {
		return nil
	}

	replicas := 1
	maxUnavailable := 1

	if replicasEnv := os.Getenv(version.ProgramUpper + "_DEPLOYMENT_REPLICAS"); replicasEnv != "" {
		if r, err := strconv.Atoi(replicasEnv); err == nil && r >= 0 {
			replicas = r
		}
	}

	if replicas > 1 {
		maxUnavailable = replicas - 1
	}

	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Labels:    pod.Labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             ptr.To(int32(replicas)),
			RevisionHistoryLimit: ptr.To(int32(0)),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       ptr.To(intstr.FromInt(1)),
					MaxUnavailable: ptr.To(intstr.FromInt(maxUnavailable)),
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: pod.Labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: pod.Labels,
				},
				Spec: pod.Spec,
			},
		},
	}
}

func (h *HostedConfig) etcdConfigSecret(e *executor.ETCDConfig, extraArgs []string, replicas int32) (*corev1.Secret, error) {
	memberList := []string{}
	for i := int32(0); i < replicas; i++ {
		hostname := fmt.Sprintf("%s-etcd-%d.%s-etcd", h.Name, i, h.Name)
		if h.Domain != "" {
			hostname = fmt.Sprintf("%s.%s.svc.%s", hostname, h.namespace, h.Domain)
		}

		memberList = append(memberList, fmt.Sprintf("%s-etcd-%d=https://%s:2380", h.Name, i, hostname))
	}

	e.InitialOptions.Cluster = strings.Join(memberList, ",")
	e.ListenClientURLs = "https://0.0.0.0:2379"
	e.ListenPeerURLs = "https://0.0.0.0:2380"
	e.ListenMetricsURLs = "http://0.0.0.0:2381"
	e.ListenClientHTTPURLs = "https://127.0.0.1:2382"
	e.DataDir = "/db/etcd"

	data := map[string][]byte{}
	for i := int32(0); i < replicas; i++ {
		hostname := fmt.Sprintf("%s-etcd-%d.%s-etcd", h.Name, i, h.Name)
		if h.Domain != "" {
			hostname = fmt.Sprintf("%s.%s.svc.%s", hostname, h.namespace, h.Domain)
		}

		e.InitialOptions.AdvertisePeerURL = fmt.Sprintf("https://%s:2380", hostname)
		e.AdvertiseClientURLs = fmt.Sprintf("https://%s:2379", hostname)
		e.Name = fmt.Sprintf("%s-etcd-%d", h.Name, i)
		b, err := etcdConfigToBytes(e, extraArgs)
		if err != nil {
			return nil, err
		}
		name := fmt.Sprintf("config.%s-etcd-%d", h.Name, i)
		data[name] = b
	}

	return &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      h.Name + "-etcd-config",
			Namespace: h.namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}, nil
}

// from executor.go ToConfigFile()
func etcdConfigToBytes(e *executor.ETCDConfig, extraArgs []string) ([]byte, error) {
	bytes, err := yaml.Marshal(&e)
	if err != nil {
		return nil, err
	}

	if len(extraArgs) > 0 {
		var s map[string]interface{}
		if err := yaml2.Unmarshal(bytes, &s); err != nil {
			return nil, err
		}

		for _, v := range extraArgs {
			extraArg := strings.SplitN(v, "=", 2)
			// Depending on the argV, we have different types to handle.
			// Source: https://github.com/etcd-io/etcd/blob/44b8ae145b505811775f5af915dd19198d556d55/server/config/config.go#L36-L190 and https://etcd.io/docs/v3.5/op-guide/configuration/#configuration-file
			if len(extraArg) == 2 {
				key := strings.TrimLeft(extraArg[0], "-")
				lowerKey := strings.ToLower(key)
				var stringArr []string
				if i, err := strconv.Atoi(extraArg[1]); err == nil {
					s[key] = i
				} else if time, err := time.ParseDuration(extraArg[1]); err == nil && (strings.Contains(lowerKey, "time") || strings.Contains(lowerKey, "duration") || strings.Contains(lowerKey, "interval") || strings.Contains(lowerKey, "retention")) {
					// auto-compaction-retention is either a time.Duration or int, depending on version. If it is an int, it will be caught above.
					s[key] = time
				} else if err := yaml.Unmarshal([]byte(extraArg[1]), &stringArr); err == nil {
					s[key] = stringArr
				} else {
					switch strings.ToLower(extraArg[1]) {
					case "true":
						s[key] = true
					case "false":
						s[key] = false
					default:
						s[key] = extraArg[1]
					}
				}
			}
		}
		bytes, err = yaml2.Marshal(&s)
		if err != nil {
			return nil, err
		}
	}
	return bytes, nil
}

func (h *HostedConfig) addSupervisorContainer(d *appsv1.Deployment) {
	envPrefix := strings.ToUpper(strings.ReplaceAll(h.Name+"-supervisor", "-", "_"))
	args := []string{
		"iptables -t nat -I PREROUTING -p tcp --dport $(" + envPrefix + "_SERVICE_PORT) -j DNAT --to $(" + envPrefix + "_SERVICE_HOST):$(" + envPrefix + "_SERVICE_PORT)",
		"iptables -t nat -I POSTROUTING -d $(" + envPrefix + "_SERVICE_HOST)/32 -p tcp -j MASQUERADE",
		"mkfifo /pause",
		"</pause",
	}
	d.Spec.Template.Spec.Containers = append(d.Spec.Template.Spec.Containers, corev1.Container{
		Name:    "supervisor",
		Image:   d.Spec.Template.Spec.Containers[0].Image,
		Command: []string{"/bin/sh", "-xec"},
		Args:    []string{strings.Join(args, "\n")},
		Ports:   []corev1.ContainerPort{{Name: "supervisor", Protocol: corev1.ProtocolTCP, ContainerPort: int32(cmds.ServerConfig.SupervisorPort)}},
		SecurityContext: &corev1.SecurityContext{
			Capabilities: &corev1.Capabilities{
				Add: []corev1.Capability{
					"NET_ADMIN",
				},
			},
		},
	})

	if d.Spec.Template.Spec.SecurityContext == nil {
		d.Spec.Template.Spec.SecurityContext = &corev1.PodSecurityContext{}
	}
	d.Spec.Template.Spec.SecurityContext.Sysctls = append(d.Spec.Template.Spec.SecurityContext.Sysctls, corev1.Sysctl{Name: "net.ipv4.ip_forward", Value: "1"})
}

// bytesFromDir returns a map of paths to bytes for normal files under a directory.
// Path separators are converted to underscores, so that the map can be used as Secret data.
// Loopback addresses in kubeconfigs are also replaced with the apiserver service name.
func (h *HostedConfig) bytesFromDir(base string) (map[string][]byte, error) {
	fileBytes := map[string][]byte{}
	base = base + string(filepath.Separator)
	return fileBytes, filepath.WalkDir(base, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// skip anything that's not a plain file
		if !d.Type().IsRegular() {
			return nil
		}
		b, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.HasSuffix(d.Name(), ".kubeconfig") {
			b = bytes.ReplaceAll(b, []byte("[::1]"), []byte(h.Name+"-kube-apiserver"))
			b = bytes.ReplaceAll(b, []byte("127.0.0.1"), []byte(h.Name+"-kube-apiserver"))
		}
		if d.Name() == "egress-selector-config.yaml" {
			b = bytes.ReplaceAll(b, []byte("[::1]"), []byte(h.Name+"-supervisor"))
			b = bytes.ReplaceAll(b, []byte("127.0.0.1"), []byte(h.Name+"-supervisor"))
		}
		path = strings.TrimPrefix(path, base)
		path = strings.ReplaceAll(path, string(filepath.Separator), "_")
		fileBytes[path] = b
		return nil
	})
}

// dirFromBytes does the opposite of bytesFromDir, extracting data from the provided
// map out to disk, with the reverse filename and content transforms applied.
// File mtimes are set to the epoch so that they are always considered older than
// whatever's in the datastore.
func (h *HostedConfig) dirFromBytes(base string, data map[string][]byte) error {
	epoch := time.Unix(0, 0)
	for path, b := range data {
		if strings.HasSuffix(path, ".kubeconfig") {
			b = bytes.ReplaceAll(b, []byte(h.Name+"-kube-apiserver"), []byte("127.0.0.1"))
		}
		if strings.HasSuffix(path, "egress-selector-config.yaml") {
			b = bytes.ReplaceAll(b, []byte(h.Name+"-supervisor"), []byte("127.0.0.1"))
		}
		path = strings.ReplaceAll(path, "_", string(filepath.Separator))
		path = filepath.Join(base, path)
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			return err
		}
		if err := os.WriteFile(path, b, 0600); err != nil {
			return err
		}
		if err := os.Chtimes(path, time.Time{}, epoch); err != nil {
			return err
		}
	}
	return nil
}
