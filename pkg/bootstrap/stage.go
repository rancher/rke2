package bootstrap

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/rancher/rke2/pkg/images"
	"github.com/sirupsen/logrus"
)

var (
	releasePattern = regexp.MustCompile("^v[0-9]")
)

func dataDirFor(dataDir, dataName string) string {
	return filepath.Join(dataDir, "data", dataName, "bin")
}

func manifestsDir(dataDir string) string {
	return filepath.Join(dataDir, "server", "manifests")
}

func symlinkBinDir(dataDir string) string {
	return filepath.Join(dataDir, "bin")
}

func dirExists(dir string) bool {
	if s, err := os.Stat(dir); err == nil && s.IsDir() {
		return true
	}
	return false
}

func Stage(dataDir string, images images.Images) (string, error) {
	ref, err := name.ParseReference(images.Runtime)
	if err != nil {
		return "", err
	}
	// downloading the image
	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", err
	}

	dataName := releaseName(ref)
	if dataName != "" {
		if dir := dataDirFor(dataDir, dataName); dirExists(dir) {
			return dir, nil
		}
	}
	if dataName == "" {
		digest, err := img.Digest()
		if err != nil {
			return "", err
		}
		dataName = digest.Hex
	}

	binDir := dataDirFor(dataDir, dataName)
	if err := extractFromDir(binDir, "/bin/", img, images.Runtime); err != nil {
		return "", err
	}

	manifestDir := manifestsDir(dataDir)
	err = extractFromDir(manifestDir, "/charts/", img, images.Runtime)

	// ignore errors
	_ = os.RemoveAll(symlinkBinDir(dataDir))
	_ = os.Symlink(binDir, symlinkBinDir(dataDir))

	return binDir, err
}

func extract(image, targetDir, prefix string, reader io.Reader) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	t := tar.NewReader(reader)
	for {
		h, err := t.Next()
		if err == io.EOF {
			logrus.Infof("Extracting %s done", image)
			return nil
		} else if err != nil {
			return err
		}

		if h.FileInfo().IsDir() {
			continue
		}

		n := filepath.Join("/", h.Name)
		if !strings.HasPrefix(n, prefix) {
			continue
		}

		targetName := filepath.Join(targetDir, filepath.Base(n))
		mode := h.FileInfo().Mode() & 0755
		f, err := os.OpenFile(targetName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return nil
		}
		logrus.Infof("Extracting %s %s...", image, h.Name)
		if _, err = io.Copy(f, t); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
}

func releaseName(ref name.Reference) string {
	if t, ok := ref.(*name.Tag); ok && releasePattern.MatchString(t.TagStr()) {
		hash := sha256.Sum256([]byte(ref.String()))
		return t.TagStr() + "-" + hex.EncodeToString(hash[:])[:12]
	} else if d, ok := ref.(*name.Digest); ok {
		str := d.DigestStr()
		parts := strings.SplitN(str, ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
		return parts[0]
	}
	return ""
}

func extractFromDir(dir, prefix string, img v1.Image, imgName string) error {
	if dirExists(dir) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}

	tempDir, err := ioutil.TempDir(filepath.Split(dir))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	r := mutate.Extract(img)
	defer r.Close()

	// extracting manifests
	if err := extract(imgName, tempDir, prefix, r); err != nil {
		return err
	}
	return os.Rename(tempDir, dir)
}
