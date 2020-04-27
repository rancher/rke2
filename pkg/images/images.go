package images

type Images struct {
	KubeAPIServer       string `json:"kube-apiserver"`
	KubeControllManager string `json:"kube-controller-manager"`
	KubeScheduler       string `json:"kube-scheduler"`
	Pause               string `json:"pause"`
	Runtime             string `json:"runtime"`
}

var (
	// These should not be used, these are settings to help with development
	//apiServer         = os.Getenv("RKE2_KUBE_APISERVER_IMAGE")
	//controllerManager = os.Getenv("RKE2_KUBE_CONTROLLER_MANAGER_IMAGE")
	//scheduler         = os.Getenv("RKE2_KUBE_SCHEDULER_IMAGE")
	//pause             = os.Getenv("RKE2_PAUSE_IMAGE")
	//runtime             = os.Getenv("RKE2_RUNTIME_IMAGE")

	apiServer         = "k8s.gcr.io/kube-apiserver:v1.18.2"
	controllerManager = "k8s.gcr.io/kube-controller-manager:v1.18.2"
	scheduler         = "k8s.gcr.io/kube-scheduler:v1.18.2"
	pause             = "k8s.gcr.io/pause:3.2"
	runtime           = "ibuildthecloud/rke2-runtime:latest"
)

func override(str, override string) string {
	if override != "" {
		return override
	}
	return str
}

func New(repo, version string) Images {
	return Images{
		KubeAPIServer:       override(repo+"/rke2-kube-apiserver:"+version, apiServer),
		KubeControllManager: override(repo+"/rke2-kube-controller-maanger:"+version, controllerManager),
		KubeScheduler:       override(repo+"/rke2-kube-scheduler:"+version, scheduler),
		Pause:               override(repo+"/rke2-pause:"+version, pause),
		Runtime:             override(repo+"/rke2-runtime:"+version, runtime),
	}
}
