package images

import (
	"os"

	"github.com/rancher/k3s/pkg/version"
)

var (
	// These should not be used, these are settings to help with development
	apiServer         = os.Getenv("RKE2_KUBE_APISERVER_IMAGE")
	controllerManager = os.Getenv("RKE2_KUBE_CONTROLLER_MANAGER_IMAGE")
	scheduler         = os.Getenv("RKE2_KUBE_SCHEDULER_IMAGE")
	pause             = os.Getenv("RKE2_PAUSE_IMAGE")
	runtime           = os.Getenv("RKE2_RUNTIME_IMAGE")
	etcd              = os.Getenv("RKE2_ETCD_IMAGE")

	KubernetesVersion = "v1.18.2"
	PauseVersion      = "3.2"
	EtcdVersion       = "3.4.3-0"
)

type Images struct {
	KubeAPIServer       string `json:"kube-apiserver"`
	KubeControllManager string `json:"kube-controller-manager"`
	KubeScheduler       string `json:"kube-scheduler"`
	Pause               string `json:"pause"`
	Runtime             string `json:"runtime"`
	ETCD                string `json:"etcd"`
}

func override(str, override string) string {
	if override != "" {
		return override
	}
	return str
}

func New(repo string) Images {
	return Images{
		KubeAPIServer:       override(override("k8s.gcr.io", repo)+"/kube-apiserver:"+KubernetesVersion, apiServer),
		KubeControllManager: override(override("k8s.gcr.io", repo)+"/kube-controller-maanger:"+KubernetesVersion, controllerManager),
		KubeScheduler:       override(override("k8s.gcr.io", repo)+"/kube-scheduler:"+KubernetesVersion, scheduler),
		Pause:               override(override("k8s.gcr.io", repo)+"/pause:"+PauseVersion, pause),
		Runtime:             override(override("rancher", repo)+"/rke2-runtime:"+version.Version, runtime),
		ETCD:                override(override("k8s.gcr.io", repo)+"/etcd:"+EtcdVersion, etcd),
	}
}
