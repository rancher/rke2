package images

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

var (
	// These should not be used, these are settings to help with development
	apiServer         = os.Getenv("RKE2_KUBE_APISERVER_IMAGE")
	controllerManager = os.Getenv("RKE2_KUBE_CONTROLLER_MANAGER_IMAGE")
	scheduler         = os.Getenv("RKE2_KUBE_SCHEDULER_IMAGE")
	pause             = os.Getenv("RKE2_PAUSE_IMAGE")
	runtime           = os.Getenv("RKE2_RUNTIME_IMAGE")
	etcd              = os.Getenv("RKE2_ETCD_IMAGE")

	KubernetesVersion = "v1.18.4"
	PauseVersion      = "3.2"
	EtcdVersion       = "v3.4.3"
	RuntimeImageName  = "rke2-runtime"
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
		KubeAPIServer:       override(override("rancher", repo)+"/kubernetes:"+KubernetesVersion, apiServer),
		KubeControllManager: override(override("rancher", repo)+"/kubernetes:"+KubernetesVersion, controllerManager),
		KubeScheduler:       override(override("rancher", repo)+"/kubernetes:"+KubernetesVersion, scheduler),
		Pause:               override(override("k8s.gcr.io", repo)+"/pause:"+PauseVersion, pause),
		Runtime:             override(override("rancher", repo)+"/rke2-runtime:"+version.Version, runtime),
		ETCD:                override(override("rancher", repo)+"/etcd:"+EtcdVersion, etcd),
	}
}

func Pull(dir, name, image string) error {
	if dir == "" {
		return nil
	}
	preloadedImages, err := checkPreloadedImages(dir)
	if err != nil {
		return err
	}
	if preloadedImages {
		return nil
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	dest := filepath.Join(dir, name+".txt")
	if err := ioutil.WriteFile(dest, []byte(image+"\n"), 0644); err != nil {
		return err
	}

	return nil
}

func checkPreloadedImages(dir string) (bool, error) {
	_, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		logrus.Errorf("unable to find images in %s: %v", dir, err)
		return false, err
	}

	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		logrus.Errorf("unable to read images in %s: %v", dir, err)
		return false, nil
	}
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}
		// the function will check for any file that doesnt end with .txt
		if !strings.HasSuffix(fileInfo.Name(), ".txt") {
			return true, nil
		}
	}
	return false, nil
}
