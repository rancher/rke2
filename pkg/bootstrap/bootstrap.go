package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	helmv1 "github.com/k3s-io/helm-controller/pkg/apis/helm.cattle.io/v1"
	"github.com/k3s-io/k3s/pkg/cli/cmds"
	"github.com/k3s-io/k3s/pkg/daemons/agent"
	daemonconfig "github.com/k3s-io/k3s/pkg/daemons/config"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/version"
	pkgerrors "github.com/pkg/errors"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/wharfie/pkg/credentialprovider/plugin"
	"github.com/rancher/wharfie/pkg/extract"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/rancher/wharfie/pkg/tarfile"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

var (
	releasePattern      = regexp.MustCompile("^v[0-9]")
	helmChartGVK        = helmv1.SchemeGroupVersion.WithKind("HelmChart")
	injectAnnotationKey = version.Program + ".cattle.io/inject-cluster-config"
	injectEnvKey        = version.ProgramUpper + "_INJECT_CLUSTER_CONFIG"
	injectDefault       = true
)

// binDirForDigest returns the path to dataDir/data/refDigest/bin.
func binDirForDigest(dataDir string, refDigest string) string {
	return filepath.Join(dataDir, "data", refDigest, "bin")
}

// chartsDirForDigest returns the path to dataDir/data/refDigest/charts.
func chartsDirForDigest(dataDir string, refDigest string) string {
	return filepath.Join(dataDir, "data", refDigest, "charts")
}

// completionMarkerForDigest returns the path to the file
// created to mark successful extract of all image content
func completionMarkerForDigest(dataDir string, refDigest string) string {
	return filepath.Join(dataDir, "data", refDigest, ".extracted")
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
	if s, err := os.Stat(dir); err == nil && s.Mode().IsDir() {
		return true
	}
	return false
}

// isRegular return true if a regular file exists at the given path.
func isRegular(file string) bool {
	if s, err := os.Stat(file); err == nil && s.Mode().IsRegular() {
		return true
	}
	return false
}

// BinDir returns the bin dir for an image by hashing the image name and tag, or the image digest,
// depending on what the runtime image reference points at.
func BinDir(resolver *images.Resolver, cfg cmds.Agent) (string, error) {
	ref, err := resolver.GetReference(images.Runtime)
	if err != nil {
		return "", err
	}

	refDigest, err := releaseRefDigest(ref)
	if err != nil {
		return "", err
	}

	return binDirForDigest(cfg.DataDir, refDigest), nil
}

// Stage extracts binaries and manifests from the runtime image specified in imageConf into the directory
// at dataDir. It attempts to load the runtime image from a tarball at dataDir/agent/images,
// falling back to a remote image pull if the image is not found within a tarball.
// Extraction is skipped if a bin directory for the specified image already exists.
// Unique image detection is accomplished by hashing the image name and tag, or the image digest,
// depending on what the runtime image reference points at.
func Stage(ctx context.Context, resolver *images.Resolver, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	var img v1.Image

	ref, err := resolver.GetReference(images.Runtime)
	if err != nil {
		return err
	}

	refDigest, err := releaseRefDigest(ref)
	if err != nil {
		return err
	}

	refBinDir := binDirForDigest(cfg.DataDir, refDigest)
	refChartsDir := chartsDirForDigest(cfg.DataDir, refDigest)
	refCompleteFile := completionMarkerForDigest(cfg.DataDir, refDigest)
	imagesDir := imagesDir(cfg.DataDir)

	if dirExists(refBinDir) && dirExists(refChartsDir) && isRegular(refCompleteFile) {
		logrus.Infof("Runtime image %s bin and charts directories already exist; skipping extract", ref.Name())
	} else {
		// Try to use configured runtime image from an airgap tarball
		img, err = preloadBootstrapFromRuntime(imagesDir, resolver)
		if err != nil {
			return err
		}

		// If we didn't find the requested image in a tarball, pull it from the remote registry.
		// Note that this will fail (potentially after a long delay) if the registry cannot be reached.
		if img == nil {
			registry, err := registries.GetPrivateRegistries(cfg.PrivateRegistry)
			if err != nil {
				return pkgerrors.WithMessagef(err, "failed to load private registry configuration from %s", cfg.PrivateRegistry)
			}
			// Override registry config with version provided by (and potentially modified by) k3s agent setup
			registry.Registry = nodeConfig.AgentConfig.Registry

			// Try to enable Kubelet image credential provider plugins; fall back to legacy docker credentials
			if agent.ImageCredProvAvailable(&nodeConfig.AgentConfig) {
				plugins, err := plugin.RegisterCredentialProviderPlugins(nodeConfig.AgentConfig.ImageCredProvConfig, nodeConfig.AgentConfig.ImageCredProvBinDir)
				if err != nil {
					return err
				}
				registry.DefaultKeychain = plugins
			} else {
				registry.DefaultKeychain = authn.DefaultKeychain
			}

			logrus.Infof("Pulling runtime image %s", ref.Name())
			// Make sure that the runtime image is also loaded into containerd
			images.Pull(imagesDir, images.Runtime, ref)
			img, err = registry.Image(ref, remote.WithPlatform(v1.Platform{Architecture: runtime.GOARCH, OS: runtime.GOOS}), remote.WithContext(ctx))
			if err != nil {
				return pkgerrors.WithMessagef(err, "failed to get runtime image %s", ref.Name())
			}
		}

		// Extract binaries and charts
		extractPaths := map[string]string{
			"/bin":    refBinDir,
			"/charts": refChartsDir,
		}
		if err := extract.ExtractDirs(img, extractPaths); err != nil {
			return pkgerrors.WithMessage(err, "failed to extract runtime image")
		}
		// Ensure correct permissions on bin dir
		if err := os.Chmod(refBinDir, 0755); err != nil {
			return err
		}
		// Create file to indicate successful extract of all content
		if err := os.WriteFile(refCompleteFile, []byte(ref.Name()), 0644); err != nil {
			return err
		}
	}

	// ignore errors on symlink rewrite
	_ = os.RemoveAll(symlinkBinDir(cfg.DataDir))
	_ = os.Symlink(refBinDir, symlinkBinDir(cfg.DataDir))

	return nil
}

// UpdateManifests copies the staged manifests into the server's manifests dir, and applies
// cluster configuration values to any HelmChart manifests found in the manifests directory.
func UpdateManifests(resolver *images.Resolver, ingressController string, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
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

	// TODO - instead of copying over then rewriting the manifests, we should template them as we
	// copy, only overwriting if they're different - and then make a second pass and rewrite any
	// user-provided manifests that weren't just copied over. This will work better with the deploy
	// controller's mtime-based change detection.

	// Copy all charts into the manifests directory, since the K3s
	// deploy controller will delete them if they are disabled.
	if err := copyDir(manifestsDir, refChartsDir); err != nil {
		return pkgerrors.WithMessage(err, "failed to copy runtime charts")
	}

	// Fix up HelmCharts to pass through configured values.
	// This needs to be done every time in order to sync values from the CLI
	if err := setChartValues(manifestsDir, ingressController, nodeConfig, cfg); err != nil {
		return pkgerrors.WithMessage(err, "failed to rewrite HelmChart manifests to pass through CLI values")
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
			logrus.Warnf("Failed to load runtime image %s from tarball: %v", ref.Name(), err)
		}
	}
	return nil, nil
}

// copyDir recursively copies files from source to destination. If the target
// file already exists, the current permissions, ownership, and xattrs will be
// retained, but the contents will be overwritten.
func copyDir(target, source string) error {
	entries, err := os.ReadDir(source)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", source, err)
	}

	if err := os.MkdirAll(target, 0755); err != nil && !os.IsExist(err) {
		return err
	}

	for _, entry := range entries {
		src := filepath.Join(source, entry.Name())
		tgt := filepath.Join(target, entry.Name())

		fileInfo, err := entry.Info()
		if err != nil {
			return fmt.Errorf("failed to get file info for %s: %w", entry.Name(), err)
		}

		switch {
		case entry.IsDir():
			if err := copyDir(tgt, src); err != nil {
				return err
			}
		case (fileInfo.Mode() & os.ModeType) == 0:
			if err := copyFile(tgt, src); err != nil {
				return err
			}
		default:
			logrus.Warnf("Skipping file with unsupported mode: %s: %s", src, fileInfo.Mode())
		}
	}
	return nil
}

// copyFile copies the the source file to the target, creating or truncating it as necessary.
func copyFile(target, source string) error {
	src, err := os.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source %s: %w", source, err)
	}
	defer src.Close()

	tgt, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open target %s: %w", target, err)
	}
	defer tgt.Close()

	_, err = io.Copy(tgt, src)
	return err
}

// setChartValues scans the directory at manifestDir. It attempts to load all manifests
// in that directory as HelmCharts. Any manifests that contain a HelmChart are modified to
// pass through settings to both the Helm job and the chart values.
// NOTE: This will probably fail if any manifest contains multiple documents. This should
// not matter for any of our packaged components, but may prevent this from working on user manifests.
func setChartValues(manifestsDir, ingressController string, nodeConfig *daemonconfig.Node, cfg cmds.Agent) error {
	chartValues := map[string]string{
		"global.clusterCIDR":                  util.JoinIPNets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterCIDRv4":                util.JoinIP4Nets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterCIDRv6":                util.JoinIP6Nets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterDNS":                   util.JoinIPs(nodeConfig.AgentConfig.ClusterDNSs),
		"global.clusterDomain":                nodeConfig.AgentConfig.ClusterDomain,
		"global.rke2DataDir":                  cfg.DataDir,
		"global.serviceCIDR":                  util.JoinIPNets(nodeConfig.AgentConfig.ServiceCIDRs),
		"global.systemDefaultIngressClass":    ingressController,
		"global.systemDefaultRegistry":        nodeConfig.AgentConfig.SystemDefaultRegistry,
		"global.cattle.systemDefaultRegistry": nodeConfig.AgentConfig.SystemDefaultRegistry,
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
		if err := rewriteChart(fileName, info, chartValues); err != nil {
			errs = append(errs, err)
		}
	}
	return merr.NewErrors(errs...)
}

// rewriteChart applies cluster configuration values to the file at fileName with associated info.
func rewriteChart(fileName string, info os.FileInfo, chartValues map[string]string) error {
	fh, err := os.OpenFile(fileName, os.O_RDWR, info.Mode())
	if err != nil {
		return pkgerrors.WithMessagef(err, "failed to open manifest %s", fileName)
	}
	defer fh.Close()

	// Ignore manifest if it cannot be decoded
	objs, err := yaml.ToObjects(fh)
	if err != nil {
		logrus.Warnf("Failed to decode manifest %s: %s", fileName, err)
		return nil
	}

	var changed bool

OBJECTS:
	for _, obj := range objs {
		// Manipulate the HelmChart using Unstructured to avoid dropping unknown fields when rewriting the content.
		// Ref: https://github.com/rancher/rke2/issues/527
		unst, ok := obj.(*unstructured.Unstructured)

		// Ignore object if it is not a HelmChart
		if !ok || unst.GroupVersionKind() != helmChartGVK {
			continue
		}

		// Ignore object if injection is disabled via annotation or default setting
		if !isInjectEnabled(unst) {
			continue
		}

		var contentChanged bool
		content := unst.UnstructuredContent()

		// Generally we should avoid using Set on HelmCharts since it cannot be overridden by HelmChartConfig,
		// but in this case we need to do it in order to avoid potentially mangling the ValuesContent YAML by
		// blindly appending content to it in order to set values.
		for k, v := range chartValues {
			cv, _, err := unstructured.NestedString(content, "spec", "set", k)
			if err != nil {
				logrus.Warnf("Failed to get current value from %s/%s in %s: %v", unst.GetNamespace(), unst.GetName(), fileName, err)
				continue OBJECTS
			}
			if cv != v {
				if err := unstructured.SetNestedField(content, v, "spec", "set", k); err != nil {
					logrus.Warnf("Failed to write chart value to %s/%s in %s: %v", unst.GetNamespace(), unst.GetName(), fileName, err)
					continue OBJECTS
				}
				contentChanged = true
			}
		}

		if contentChanged {
			changed = true
			unst.SetUnstructuredContent(content)
		}
	}

	if !changed {
		logrus.Infof("No cluster configuration value changes necessary for manifest %s", fileName)
		return nil
	}

	data, err := yaml.Export(objs...)
	if err != nil {
		return pkgerrors.WithMessagef(err, "failed to export modified manifest %s", fileName)
	}

	if _, err := fh.Seek(0, 0); err != nil {
		return pkgerrors.WithMessagef(err, "failed to seek in manifest %s", fileName)
	}

	if err := fh.Truncate(0); err != nil {
		return pkgerrors.WithMessagef(err, "failed to truncate manifest %s", fileName)
	}

	if _, err := fh.Write(data); err != nil {
		return pkgerrors.WithMessagef(err, "failed to write modified manifest %s", fileName)
	}

	if err := fh.Sync(); err != nil {
		return pkgerrors.WithMessagef(err, "failed to sync modified manifest %s", fileName)
	}

	logrus.Infof("Updated manifest %s to set cluster configuration values", fileName)
	return nil
}

func isInjectEnabled(obj *unstructured.Unstructured) bool {
	if v, ok := obj.GetAnnotations()[injectAnnotationKey]; ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return getInjectDefault()
}

func getInjectDefault() bool {
	if b, err := strconv.ParseBool(os.Getenv(injectEnvKey)); err == nil {
		return b
	}
	return injectDefault
}
