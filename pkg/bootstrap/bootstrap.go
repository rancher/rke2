package bootstrap

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/containerd/continuity/fs"
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/cli/cmds"
	"github.com/rancher/k3s/pkg/daemons/agent"
	daemonconfig "github.com/rancher/k3s/pkg/daemons/config"
	"github.com/rancher/k3s/pkg/util"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/wharfie/pkg/credentialprovider/plugin"
	"github.com/rancher/wharfie/pkg/extract"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/rancher/wharfie/pkg/tarfile"
	"github.com/rancher/wrangler/pkg/merr"
	"github.com/rancher/wrangler/pkg/schemes"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	releasePattern = regexp.MustCompile("^v[0-9]")
	ps             = string(os.PathSeparator)
)

// binDirForDigest returns the path to dataDir/data/refDigest/bin.
func binDirForDigest(dataDir string, refDigest string) string {
	return filepath.Join(dataDir, "data", refDigest, "bin")
}

// chartsDirForDigest returns the path to dataDir/data/refDigest/charts.
func chartsDirForDigest(dataDir string, refDigest string) string {
	return filepath.Join(dataDir, "data", refDigest, "charts")
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

// Stage extracts binaries and manifests from the runtime image specified in imageConf into the directory
// at dataDir. It attempts to load the runtime image from a tarball at dataDir/agent/images,
// falling back to a remote image pull if the image is not found within a tarball.
// Extraction is skipped if a bin directory for the specified image already exists.
// Unique image detection is accomplished by hashing the image name and tag, or the image digest,
// depending on what the runtime image reference points at.
// If the bin directory already exists, or content is successfully extracted, the bin directory path is returned.
func Stage(resolver *images.Resolver, nodeConfig *daemonconfig.Node, cfg cmds.Agent) (string, error) {
	var img v1.Image

	ref, err := resolver.GetReference(images.Runtime)
	if err != nil {
		return "", err
	}

	refDigest, err := releaseRefDigest(ref)
	if err != nil {
		return "", err
	}

	refBinDir := binDirForDigest(cfg.DataDir, refDigest)
	refChartsDir := chartsDirForDigest(cfg.DataDir, refDigest)
	imagesDir := imagesDir(cfg.DataDir)

	if dirExists(refBinDir) && dirExists(refChartsDir) {
		logrus.Infof("Runtime image %s bin and charts directories already exist; skipping extract", ref.Name())
	} else {
		// Try to use configured runtime image from an airgap tarball
		img, err = preloadBootstrapFromRuntime(imagesDir, resolver)
		if err != nil {
			return "", err
		}

		// If we didn't find the requested image in a tarball, pull it from the remote registry.
		// Note that this will fail (potentially after a long delay) if the registry cannot be reached.
		if img == nil {
			registry, err := registries.GetPrivateRegistries(nodeConfig.AgentConfig.PrivateRegistry)
			if err != nil {
				return "", errors.Wrapf(err, "failed to load private registry configuration from %s", nodeConfig.AgentConfig.PrivateRegistry)
			}

			// Prefer registries.yaml auth config
			kcs := []authn.Keychain{registry}

			// Try to enable Kubelet image credential provider plugins; fall back to legacy docker credentials
			if agent.ImageCredProvAvailable(&nodeConfig.AgentConfig) {
				plugins, err := plugin.RegisterCredentialProviderPlugins(nodeConfig.AgentConfig.ImageCredProvConfig, nodeConfig.AgentConfig.ImageCredProvBinDir)
				if err != nil {
					return "", err
				}
				kcs = append(kcs, plugins)
			} else {
				kcs = append(kcs, authn.DefaultKeychain)
			}

			multiKeychain := authn.NewMultiKeychain(kcs...)

			logrus.Infof("Pulling runtime image %s", ref.Name())
			img, err = remote.Image(registry.Rewrite(ref),
				remote.WithAuthFromKeychain(multiKeychain),
				remote.WithTransport(registry),
				remote.WithPlatform(v1.Platform{
					Architecture: runtime.GOARCH,
					OS:           runtime.GOOS,
				}),
			)
			if err != nil {
				return "", errors.Wrapf(err, "failed to get runtime image %s", ref.Name())
			}
		}

		// Extract binaries and charts
		extractPaths := map[string]string{
			ps + "bin":    refBinDir,
			ps + "charts": refChartsDir,
		}
		if err := extract.ExtractDirs(img, extractPaths); err != nil {
			return "", errors.Wrap(err, "failed to extract runtime image")
		}
		// Ensure correct permissions on bin dir
		if err := os.Chmod(refBinDir, 0755); err != nil {
			return "", err
		}
	}

	// ignore errors on symlink rewrite
	_ = os.RemoveAll(symlinkBinDir(cfg.DataDir))
	_ = os.Symlink(refBinDir, symlinkBinDir(cfg.DataDir))

	return refBinDir, nil
}

// UpdateManifests copies the staged manifests into the server's manifests dir, and applies
// cluster configuration values to any HelmChart manifests found in the manifests directory.
func UpdateManifests(resolver *images.Resolver, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	ref, err := resolver.GetReference(images.Runtime)
	if err != nil {
		return err
	}

	refDigest, err := releaseRefDigest(ref)
	if err != nil {
		return err
	}

	refChartsDir := chartsDirForDigest(cfg.DataDir, refDigest)
	manifestsDir := manifestsDir(cfg.DataDir)

	// Ensure manifests directory exists
	if err := os.MkdirAll(manifestsDir, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	// TODO - instead of copying over then rewriting the manifests, we should template them as we
	// copy, only overwriting if they're different - and then make a second pass and rewrite any
	// user-provided manifests that weren't just copied over. This will work better with the deploy
	// controller's mtime-based change detection.

	// Recursively copy all charts into the manifests directory, since the K3s
	// deploy controller will delete them if they are disabled.
	if err := fs.CopyDir(manifestsDir, refChartsDir); err != nil {
		return errors.Wrap(err, "failed to copy runtime charts")
	}

	// Fix up HelmCharts to pass through configured values.
	// This needs to be done every time in order to sync values from the CLI
	if err := setChartValues(manifestsDir, nodeConfig, cfg); err != nil {
		return errors.Wrap(err, "failed to rewrite HelmChart manifests to pass through CLI values")
	}
	return nil
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
	return "", fmt.Errorf("Runtime image %s is not a not a reference to a digest or version tag matching pattern %s", ref.Name(), releasePattern)
}

// preloadBootstrapFromRuntime tries to load the runtime image from tarballs, using both the
// default registry, and the user-configured registry (on the off chance they've retagged the
// images in the tarball to match their private registry).
func preloadBootstrapFromRuntime(imagesDir string, resolver *images.Resolver) (v1.Image, error) {
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
		img, err := tarfile.FindImage(imagesDir, ref)
		if img != nil {
			return img, err
		}
		if err != nil {
			logrus.Errorf("Failed to load runtime image %s: %v", ref.Name(), err)
		}
	}
	return nil, nil
}

// setChartValues scans the directory at manifestDir. It attempts to load all manifests
// in that directory as HelmCharts. Any manifests that contain a HelmChart are modified to
// pass through settings to both the Helm job and the chart values.
// NOTE: This will probably fail if any manifest contains multiple documents. This should
// not matter for any of our packaged components, but may prevent this from working on user manifests.
func setChartValues(manifestsDir string, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	serializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, schemes.All, schemes.All, json.SerializerOptions{Yaml: true, Pretty: true, Strict: true})
	chartValues := map[string]string{
		"global.clusterCIDR":           util.JoinIPNets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterCIDRv4":         util.JoinIP4Nets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterCIDRv6":         util.JoinIP6Nets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterDNS":            util.JoinIPs(nodeConfig.AgentConfig.ClusterDNSs),
		"global.clusterDomain":         nodeConfig.AgentConfig.ClusterDomain,
		"global.rke2DataDir":           cfg.DataDir,
		"global.serviceCIDR":           util.JoinIPNets(nodeConfig.AgentConfig.ServiceCIDRs),
		"global.systemDefaultRegistry": nodeConfig.AgentConfig.SystemDefaultRegistry,
	}

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
		if err := rewriteChart(fileName, info, chartValues, serializer); err != nil {
			errs = append(errs, err)
		}
	}
	return merr.NewErrors(errs...)
}

// rewriteChart applies cluster configuration values to the file at fileName with associated info.
// If the file cannot be decoded as a HelmChart, it is silently skipped. Any other IO error is considered
// a failure.
func rewriteChart(fileName string, info os.FileInfo, chartValues map[string]string, serializer *json.Serializer) error {
	bytes, err := ioutil.ReadFile(fileName)
	if err != nil {
		return errors.Wrapf(err, "Failed to read manifest %s", fileName)
	}

	// Ignore manifest if it cannot be decoded
	obj, _, err := serializer.Decode(bytes, nil, nil)
	if err != nil {
		logrus.Debugf("Failed to decode manifest %s: %s", fileName, err)
		return nil
	}

	// Ignore manifest if it is not a HelmChart
	chart, ok := obj.(*helmv1.HelmChart)
	if !ok {
		logrus.Debugf("Manifest %s is %T, not HelmChart", fileName, obj)
		return nil
	}

	// Generally we should avoid using Set on HelmCharts since it cannot be overridden by HelmChartConfig,
	// but in this case we need to do it in order to avoid potentially mangling the ValuesContent field by
	// blindly appending content to it in order to set values.
	if chart.Spec.Set == nil {
		chart.Spec.Set = map[string]intstr.IntOrString{}
	}

	for k, v := range chartValues {
		chart.Spec.Set[k] = intstr.FromString(v)
	}

	f, err := os.OpenFile(fileName, os.O_RDWR|os.O_TRUNC, info.Mode())
	if err != nil {
		return errors.Wrapf(err, "Unable to open HelmChart %s", fileName)
	}

	if err := serializer.Encode(chart, f); err != nil {
		_ = f.Close()
		return errors.Wrapf(err, "Failed to serialize modified HelmChart %s", fileName)
	}

	if err := f.Close(); err != nil {
		return errors.Wrapf(err, "Failed to write modified HelmChart %s", fileName)
	}

	logrus.Infof("Updated HelmChart %s to set cluster configuration values", fileName)
	return nil
}
