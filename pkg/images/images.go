package images

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rancher/k3s/pkg/version"
	"github.com/sirupsen/logrus"
)

const (
	dockerRegistry = "docker.io"
)

var (
	KubernetesVersion = "v1.19.7"      // make sure this matches what is in the scripts/version.sh script
	PauseVersion      = "3.2"          // make sure this matches what is in the scripts/build-images script
	EtcdVersion       = "v3.4.13-k3s1" // make sure this matches what is in the scripts/build-images script
	RuntimeImageName  = "rke2-runtime"
)

type Images struct {
	SystemDefaultRegistry string `json:"system-default-registry"`
	Runtime               string `json:"runtime"`
	KubeAPIServer         string `json:"kube-apiserver"`
	KubeControllManager   string `json:"kube-controller-manager"`
	KubeScheduler         string `json:"kube-scheduler"`
	ETCD                  string `json:"etcd"`
	Pause                 string `json:"pause"`
}

// override returns the override value if it's not an empty string (after trimming), or the default if it is empty.
func override(defaultValue string, overrideValue string) string {
	overrideValue = strings.TrimSpace(overrideValue)
	if overrideValue != "" {
		return overrideValue
	}
	return defaultValue
}

// SetDefaults updates the image list, honoring the SystemDefaultRegistry and Image overrides if they are not empty.
func (i *Images) SetDefaults() {
	i.Runtime = override(override(dockerRegistry, i.SystemDefaultRegistry)+"/rancher/"+RuntimeImageName+":"+strings.ReplaceAll(version.Version, "+", "-"), i.Runtime)
	i.KubeAPIServer = override(override(dockerRegistry, i.SystemDefaultRegistry)+"/rancher/hardened-kubernetes:"+KubernetesVersion, i.KubeAPIServer)
	i.KubeControllManager = override(override(dockerRegistry, i.SystemDefaultRegistry)+"/rancher/hardened-kubernetes:"+KubernetesVersion, i.KubeControllManager)
	i.KubeScheduler = override(override(dockerRegistry, i.SystemDefaultRegistry)+"/rancher/hardened-kubernetes:"+KubernetesVersion, i.KubeScheduler)
	i.ETCD = override(override(dockerRegistry, i.SystemDefaultRegistry)+"/rancher/hardened-etcd:"+EtcdVersion, i.ETCD)
	i.Pause = override(override(dockerRegistry, i.SystemDefaultRegistry)+"/rancher/pause:"+PauseVersion, i.Pause)
}

// Pull checks for preloaded images in dir. If they are available, nothing is done.
// If they are not available, it adds the image to name.txt in dir.
// This is mostly used to track what images are being pulled for static pods.
func Pull(dir, name, image string) error {
	if dir == "" {
		return nil
	}

	if imagesExist, err := checkPreloadedImages(dir); err != nil {
		return err
	} else if imagesExist {
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

// checkPreloadedImages returns true if there are any files in dir that do not
// end with a .txt extension. The presence of at least one such file is presumed to
// indicate that there is an airgap image tarball available.
func checkPreloadedImages(dir string) (bool, error) {
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		logrus.Errorf("unable to stat image directory %s: %v", dir, err)
		return false, err
	}

	fileInfos, err := ioutil.ReadDir(dir)
	if err != nil {
		logrus.Errorf("unable to list images in %s: %v", dir, err)
		return false, nil
	}
	for _, fileInfo := range fileInfos {
		if fileInfo.IsDir() {
			continue
		}
		// return true if there is a file that doesn't end with .txt
		if !strings.HasSuffix(fileInfo.Name(), ".txt") {
			return true, nil
		}
	}
	return false, nil
}
