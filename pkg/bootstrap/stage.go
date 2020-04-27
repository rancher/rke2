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
	"github.com/rancher/rke2/pkg/images"
	"github.com/sirupsen/logrus"
)

var (
	releasePattern = regexp.MustCompile("^v[0-9]")
)

func dataDirFor(dataDir, dataName string) string {
	return filepath.Join(dataDir, "data", dataName, "bin")
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

	dataName := releaseName(ref)
	if dataName != "" {
		if dir := dataDirFor(dataDir, dataName); dirExists(dir) {
			return dir, nil
		}
	}

	img, err := remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return "", err
	}

	if dataName == "" {
		digest, err := img.Digest()
		if err != nil {
			return "", err
		}
		dataName = digest.Hex
	}

	dir := dataDirFor(dataDir, dataName)
	if dirExists(dir) {
		return dir, nil
	}

	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return "", err
	}

	tempDir, err := ioutil.TempDir(filepath.Split(dir))
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tempDir)

	r := mutate.Extract(img)
	defer r.Close()

	if err := extract(images.Runtime, tempDir, r); err != nil {
		return "", err
	}

	return dir, os.Rename(tempDir, dir)
}

func extract(image, targetDir string, reader io.Reader) error {
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
		if !strings.HasPrefix(n, "/bin/") {
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
