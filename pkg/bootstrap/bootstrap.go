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
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	errors2 "github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/sirupsen/logrus"
)

var releasePattern = regexp.MustCompile("^v[0-9]")

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
	// preload image from tarball
	img, err := preloadBootstrapImage(dataDir, images.Runtime)
	if err != nil {
		return "", err
	}
	if img == nil {
		// downloading the image
		img, err = remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
		if err != nil {
			return "", err
		}
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
	if err := os.Chmod(binDir, 0755); err != nil {
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

	// we're ignoring any returned errors since the likelihood is that
	// the error is that the new path already exists. That's indicative of a
	// previously bootstrapped system. If it's a different error, it's indicative
	// of an operating system or filesystem issue.
	if err := os.Rename(tempDir, dir); err == nil {
		return nil
	}

	// manifests dir exists
	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		return err
	}

	var errs []error
	for _, file := range files {
		src := filepath.Join(tempDir, file.Name())
		dst := filepath.Join(dir, file.Name())
		if err := os.Rename(src, dst); err == os.ErrExist {
			if err = os.Remove(dst); err != nil {
				errs = append(errs, errors2.Wrapf(err, "failed to remove file %s", dst))
				continue
			}
			if err = os.Rename(src, dst); err != nil {
				errs = append(errs, errors2.Wrapf(err, "failed to rename file %s to %s", src, dst))
			}
		} else if err != nil {
			errs = append(errs, errors2.Wrapf(err, "failed to move file %s", src))
		}
	}
	if len(errs) > 0 {
		return merr.NewErrors(errs...)
	}
	return nil
}

func preloadBootstrapImage(dataDir, runtimeImage string) (v1.Image, error) {
	imagesDir := filepath.Join(dataDir, "agent", "images")
	if _, err := os.Stat(imagesDir); err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	files := map[string]os.FileInfo{}
	if err := filepath.Walk(imagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		files[path] = info
		return nil
	}); err != nil {
		return nil, err
	}
	archTag, err := name.NewTag(runtimeImage, name.WeakValidation)
	if err != nil {
		return nil, err
	}
	for fileName := range files {
		logrus.Infof("Attempting bootstrap from %s ...", fileName)
		img, err := tarball.ImageFromPath(fileName, &archTag)
		if err != nil {
			continue
		}
		return img, nil

	}
	return nil, nil
}
