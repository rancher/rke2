package bootstrap

import (
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	"github.com/k3s-io/helm-controller/pkg/helm"
	errors2 "github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/rancher/wrangler/pkg/schemes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var releasePattern = regexp.MustCompile("^v[0-9]")

const bufferSize = 4096

// binDirForDigest returns the path to dataDir/data/refDigest/bin.
func binDirForDigest(dataDir string, refDigest string) string {
	return filepath.Join(dataDir, "data", refDigest, "bin")
}

// manifestsDir returns the path to dataDir/server/manifests.
func manifestsDir(dataDir string) string {
	return filepath.Join(dataDir, "server", "manifests")
}

// imagesDir returns the path to dataDir/agent/images.
func imagesDir(dataDir string) string {
	return filepath.Join(dataDir, "agent", "images")
}

// symlinkBinDir returns the path to dataDir/bin.
// This will be symlinked to the current runtime bin dir.
func symlinkBinDir(dataDir string) string {
	return filepath.Join(dataDir, "bin")
}

// dirExists returns true if a directory exists at the given path.
func dirExists(dir string) bool {
	if s, err := os.Stat(dir); err == nil && s.IsDir() {
		return true
	}
	return false
}

// Stage extracts binaries and manifests from the runtime image specified in the image configuration
// into the directory at dataDir. It attempts to load the runtime image from a tarball at
// dataDir/agent/images, falling back to a remote image pull if the image is not found within a
// tarball.  Extraction is skipped if a bin directory for the specified image already exists. It
// also rewrites any HelmCharts to pass through the --system-default-registry value.  Unique image
// detection is accomplished by hashing the image name and tag, or the image digest, depending on
// what the runtime image reference points at.  If the bin directory already exists, or content is
// successfully extracted, the bin directory path is returned.
func Stage(dataDir, privateRegistry string, resolver *images.Resolver) (string, error) {
	var img v1.Image

	ref, err := resolver.GetReference(images.Runtime)
	if err != nil {
		return "", err
	}

	refDigest, err := releaseRefDigest(ref)
	if err != nil {
		return "", err
	}

	refBinDir := binDirForDigest(dataDir, refDigest)
	manifestsDir := manifestsDir(dataDir)

	// Skip content extraction if the bin dir for this runtime image already exists
	if dirExists(refBinDir) {
		logrus.Infof("Runtime image %q bin dir already exists at %q; skipping extract", ref, refBinDir)
	} else {
		// Try to use configured runtime image from an airgap tarball
		img, err = preloadBootstrapFromRuntime(dataDir, resolver)
		if err != nil {
			return "", err
		}

		// If we didn't find the requested image in a tarball, pull it from the remote registry.
		// Note that this will fail (potentially after a long delay) if the registry cannot be reached.
		if img == nil {
			logrus.Infof("Pulling runtime image %q", ref)
			img, err = remote.Image(ref, remote.WithAuthFromKeychain(authn.DefaultKeychain))
			if err != nil {
				return "", errors2.Wrapf(err, "Failed to pull runtime image %q", ref)
			}
		}

		// Extract binaries
		if err := extractToDir(refBinDir, "/bin/", img, ref.String()); err != nil {
			return "", err
		}
		if err := os.Chmod(refBinDir, 0755); err != nil {
			return "", err
		}

		// Extract charts to manifests dir
		if err := extractToDir(manifestsDir, "/charts/", img, ref.String()); err != nil {
			return "", err
		}
	}

	// Fix up HelmCharts to pass through configured values
	// This needs to be done every time in order to sync values from the CLI
	if err := setChartValues(dataDir, resolver.Registry.Name()); err != nil {
		return "", err
	}

	// ignore errors on symlink rewrite
	_ = os.RemoveAll(symlinkBinDir(dataDir))
	_ = os.Symlink(refBinDir, symlinkBinDir(dataDir))

	return refBinDir, nil
}

// extract extracts image content to targetDir all content from reader where the filename is prefixed with prefix.
// The imageName argument is used solely for logging.
// The destination directory is expected to be nonexistent or empty.
func extract(imageName string, targetDir string, prefix string, reader io.Reader) error {
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return err
	}

	t := tar.NewReader(reader)
	for {
		h, err := t.Next()
		if err == io.EOF {
			logrus.Infof("Done extracting %q", imageName)
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

		logrus.Infof("Extracting file %q", h.Name)

		targetName := filepath.Join(targetDir, filepath.Base(n))
		mode := h.FileInfo().Mode() & 0755
		f, err := os.OpenFile(targetName, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
		if err != nil {
			return err
		}

		if _, err = io.Copy(f, t); err != nil {
			f.Close()
			return err
		}
		if err := f.Close(); err != nil {
			return err
		}
	}
}

// releaseRefDigest returns a unique name for an image reference.
// If the image refers to a tag that appears to be a version string, it returns the tag + the first 12 bytes of the SHA256 hash of the reference string.
// If the image refers to a digest, it returns the digest, without the alg prefix ("sha256:", etc).
// If neither of the above conditions are met (semver tag or digest), an error is raised.
func releaseRefDigest(ref name.Reference) (string, error) {
	if t, ok := ref.(name.Tag); ok && releasePattern.MatchString(t.TagStr()) {
		hash := sha256.Sum256([]byte(ref.String()))
		return t.TagStr() + "-" + hex.EncodeToString(hash[:])[:12], nil
	} else if d, ok := ref.(name.Digest); ok {
		str := d.DigestStr()
		parts := strings.SplitN(str, ":", 2)
		if len(parts) == 2 {
			return parts[1], nil
		}
		return parts[0], nil
	}
	return "", fmt.Errorf("Bootstrap image %q is not a not a reference to a digest or version tag (%q)", ref, releasePattern)
}

// extractToDir extracts to targetDir all content from img where the filename is prefixed with prefix.
// The imageName argument is used solely for logging.
// Extracted content is staged through a temporary directory and moved into place, overwriting any existing files.
func extractToDir(dir, prefix string, img v1.Image, imageName string) error {
	logrus.Infof("Extracting %q %q to %q", imageName, prefix, dir)
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return err
	}

	tempDir, err := ioutil.TempDir(filepath.Split(dir))
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	imageReader := mutate.Extract(img)
	defer imageReader.Close()

	// Extract content to temporary directory.
	if err := extract(imageName, tempDir, prefix, imageReader); err != nil {
		return err
	}

	// Try to rename the temp dir into its target location.
	if err := os.Rename(tempDir, dir); err == nil {
		// Successfully renamed into place, nothing else to do.
		return nil
	} else if !os.IsExist(err) {
		// Failed to rename, but not because the destination already exists.
		return err
	}

	// Target directory already exists (got ErrExist above), fall back list/rename files into place.
	files, err := ioutil.ReadDir(tempDir)
	if err != nil {
		return err
	}

	var errs []error
	for _, file := range files {
		src := filepath.Join(tempDir, file.Name())
		dst := filepath.Join(dir, file.Name())
		if err := os.Rename(src, dst); os.IsExist(err) {
			// Can't rename because dst already exists, remove it...
			if err = os.RemoveAll(dst); err != nil {
				errs = append(errs, errors2.Wrapf(err, "failed to remove %q", dst))
				continue
			}
			// ...then try renaming again
			if err = os.Rename(src, dst); err != nil {
				errs = append(errs, errors2.Wrapf(err, "failed to rename %q to %q", src, dst))
			}
		} else if err != nil {
			// Other error while renaming src to dst.
			errs = append(errs, errors2.Wrapf(err, "failed to rename %q to %q", src, dst))
		}
	}
	return merr.NewErrors(errs...)
}

// preloadBootstrapFromRuntime tries to load the runtime image from tarballs, using both the
// default registry, and the user-configured registry (on the off chance they've retagged the
// images in the tarball to match their private registry).
func preloadBootstrapFromRuntime(dataDir string, resolver *images.Resolver) (v1.Image, error) {
	var refs []name.Reference
	runtimeRef, err := resolver.GetReference(images.Runtime)
	if err != nil {
		return nil, err
	}

	if runtimeRef.Context().Registry.Name() == images.DefaultRegistry {
		// If the image is from the default registry, only check for that.
		refs = []name.Reference{runtimeRef}
	} else {
		// If the image is from a different registry, check the default first, then the configured registry.
		defaultRef, err := resolver.GetReference(images.Runtime, images.WithRegistry(images.DefaultRegistry))
		if err != nil {
			return nil, err
		}
		refs = []name.Reference{defaultRef, runtimeRef}
	}

	for _, ref := range refs {
		img, err := preloadBootstrapImage(dataDir, ref)
		if img != nil {
			return img, err
		}
		if err != nil {
			logrus.Errorf("Failed to load for bootstrap image %s: %v", ref.Name(), err)
		}
	}
	return nil, nil
}

// preloadBootstrapImage attempts return an image matching the given reference from a tarball
// within imagesDir.
func preloadBootstrapImage(dataDir string, imageRef name.Reference) (v1.Image, error) {
	imageTag, ok := imageRef.(name.Tag)
	if !ok {
		logrus.Debugf("No local image available for %s: reference is not a tag", imageRef)
		return nil, nil
	}

	imagesDir := imagesDir(dataDir)
	if _, err := os.Stat(imagesDir); err != nil {
		if os.IsNotExist(err) {
			logrus.Debugf("No local image available for %s: directory %s does not exist", imageTag, imagesDir)
			return nil, nil
		}
		return nil, err
	}

	// Walk the images dir to get a list of tar files
	files := map[string]os.FileInfo{}
	if err := filepath.Walk(imagesDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(path, ".tar") {
			files[path] = info
		}
		return nil
	}); err != nil {
		return nil, err
	}

	// Try to find the requested tag in each file, moving on to the next if there's an error
	for fileName := range files {
		img, err := tarball.ImageFromPath(fileName, &imageTag)
		if err != nil {
			logrus.Debugf("Did not find %s in %s: %s", imageTag, fileName, err)
			continue
		}
		logrus.Debugf("Found %s in %s", imageTag, fileName)
		return img, nil
	}
	logrus.Debugf("No local image available for %s: not found in any file in %s", imageTag, imagesDir)
	return nil, nil
}

// setChartValues scans the directory at manifestDir. It attempts to load all manifests
// in that directory as HelmCharts. Any manifests that contain a HelmChart are modified to
// pass through settings to both the Helm job and the chart values.
// NOTE: This will probably fail if any manifest contains multiple documents. This should
// not matter for any of our packaged components, but may prevent this from working on user manifests.
func setChartValues(dataDir string, systemDefaultRegistry string) error {
	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, schemes.All, schemes.All, json.SerializerOptions{Yaml: true, Pretty: true, Strict: true})
	manifestsDir := manifestsDir(dataDir)

	files := map[string]os.FileInfo{}
	if err := filepath.Walk(manifestsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		switch {
		case info.IsDir():
			return nil
		case strings.HasSuffix(path, ".yml"):
		case strings.HasSuffix(path, ".yaml"):
		default:
			return nil
		}
		files[path] = info
		return nil
	}); err != nil {
		return err
	}

	var errs []error
	for fileName, info := range files {
		if err := rewriteChart(fileName, info, dataDir, systemDefaultRegistry, serializer); err != nil {
			errs = append(errs, err)
		}
	}
	return merr.NewErrors(errs...)
}

// rewriteChart applies dataDir and systemDefaultRegistry settings to the file at fileName with associated info.
// If the file cannot be decoded as a HelmChart, it is silently skipped. Any other IO error is considered
// a failure.
func rewriteChart(fileName string, info os.FileInfo, dataDir, systemDefaultRegistry string, serializer *json.Serializer) error {
	chartChanged := false

	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors2.Wrapf(err, "Failed to read manifest %q", fileName)
	}

	// Ignore manifest if it cannot be decoded
	obj, _, err := serializer.Decode(bytes, nil, nil)
	if err != nil {
		logrus.Debugf("Failed to decode manifest %q: %s", fileName, err)
		return nil
	}

	// Ignore manifest if it is not a HelmChart
	chart, ok := obj.(*helmv1.HelmChart)
	if !ok {
		logrus.Debugf("Manifest %q is %T, not HelmChart", fileName, obj)
		return nil
	}

	// Generally we should avoid using Set on HelmCharts since it cannot be overridden by HelmChartConfig,
	// but in this case we need to do it in order to avoid potentially mangling the ValuesContent field by
	// blindly appending content to it in order to set values.
	if chart.Spec.Set == nil {
		chart.Spec.Set = map[string]intstr.IntOrString{}
	}

	if chart.Spec.Set["global.rke2DataDir"].StrVal != dataDir {
		chart.Spec.Set["global.rke2DataDir"] = intstr.FromString(dataDir)
		chartChanged = true
	}

	if chart.Spec.Set["global.systemDefaultRegistry"].StrVal != systemDefaultRegistry {
		chart.Spec.Set["global.systemDefaultRegistry"] = intstr.FromString(systemDefaultRegistry)
		chartChanged = true
	}

	jobImage := helm.DefaultJobImage
	if systemDefaultRegistry != "" {
		jobImage = systemDefaultRegistry + "/" + helm.DefaultJobImage
	}

	if chart.Spec.JobImage != jobImage {
		chart.Spec.JobImage = jobImage
		chartChanged = true
	}

	if chartChanged {
		f, err := os.OpenFile(fileName, os.O_RDWR|os.O_TRUNC, info.Mode())
		if err != nil {
			return errors2.Wrapf(err, "Unable to open HelmChart %q", fileName)
		}

		if err := serializer.Encode(chart, f); err != nil {
			_ = f.Close()
			return errors2.Wrapf(err, "Failed to serialize modified HelmChart %q", fileName)
		}

		if err := f.Close(); err != nil {
			return errors2.Wrapf(err, "Failed to write modified HelmChart %q", fileName)
		}

		logrus.Infof("Updated HelmChart %q to apply --system-default-registry modifications", fileName)
	}
	return nil
}
