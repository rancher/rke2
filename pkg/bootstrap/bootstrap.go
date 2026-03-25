package bootstrap

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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
	"github.com/k3s-io/k3s/pkg/signals"
	"github.com/k3s-io/k3s/pkg/util"
	"github.com/k3s-io/k3s/pkg/util/errors"
	"github.com/k3s-io/k3s/pkg/version"
	"github.com/otiai10/copy"
	"github.com/rancher/rke2/pkg/cli"
	"github.com/rancher/rke2/pkg/images"
	"github.com/rancher/wharfie/pkg/credentialprovider/plugin"
	"github.com/rancher/wharfie/pkg/extract"
	"github.com/rancher/wharfie/pkg/registries"
	"github.com/rancher/wharfie/pkg/tarfile"
	"github.com/rancher/wrangler/v3/pkg/merr"
	"github.com/rancher/wrangler/v3/pkg/yaml"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
)

var (
	releasePattern      = regexp.MustCompile("^v[0-9]")
	helmChartGVK        = helmv1.SchemeGroupVersion.WithKind("HelmChart")
	injectAnnotationKey = version.Program + ".cattle.io/inject-cluster-config"
	injectEnvKey        = version.ProgramUpper + "_INJECT_CLUSTER_CONFIG"
	injectDefault       = false
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

// dirHasRunningProcesses returns true if any running process has its executable rooted inside dirPath.
func dirHasRunningProcesses(dirPath string) bool {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return false
	}
	prefix := dirPath + "/"
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if _, err := strconv.Atoi(entry.Name()); err != nil {
			continue
		}
		exe, err := os.Readlink(filepath.Join("/proc", entry.Name(), "exe"))
		if err != nil {
			continue
		}
		if strings.HasPrefix(exe, prefix) {
			return true
		}
	}
	return false
}

// cleanDataDirs remove directories from /data that do not match the current version/refdigest.
func cleanDataDirs(dataDir string, refDigest string) error {
	datapath := filepath.Join(dataDir, "data")
	entries, err := os.ReadDir(datapath)

	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if entry.Name() == refDigest {
			continue
		}
		if !releasePattern.MatchString(entry.Name()) {
			continue
		}

		oldpath := filepath.Join(datapath, entry.Name())
		if dirHasRunningProcesses(oldpath) {
			logrus.Infof("Skipping removal of old RKE2 data directory %s: processes still running from it", oldpath)
			continue
		}
		logrus.Infof("Removing old RKE2 data directory: %s", oldpath)
		if err := os.RemoveAll(oldpath); err != nil {
			logrus.Warnf("Failed to removed old data directory %s: %v", oldpath, err)
		}
	}
	return nil
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
				return errors.WithMessagef(err, "failed to load private registry configuration from %s", cfg.PrivateRegistry)
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
				return errors.WithMessagef(err, "failed to get runtime image %s", ref.Name())
			}
		}

		// Extract binaries and charts
		extractPaths := map[string]string{
			"/bin":    refBinDir,
			"/charts": refChartsDir,
		}
		if err := extract.ExtractDirs(img, extractPaths); err != nil {
			return errors.WithMessage(err, "failed to extract runtime image")
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

	if err := cleanDataDirs(cfg.DataDir, refDigest); err != nil {
		logrus.Warnf("Failed to clean old data dirs: %v", err)
	}

	return nil
}

// UpdateManifests copies the staged manifests into the server's manifests dir, and applies
// cluster configuration values to any HelmChart manifests found in the manifests directory.
// This function is intended to be run in a goroutine, and will block until the apiserver is up to list HelmCharts.
// TODO: Move this function back out of a goroutine once we no longer support detecting legacy installations of ingress-nginx
func UpdateManifests(ctx context.Context, resolver *images.Resolver, ingressController []string, nodeConfig *daemonconfig.Node, cfg cmds.Agent, prime bool) {
	if err := updateManifests(ctx, resolver, ingressController, nodeConfig, cfg, prime); err != nil {
		signals.RequestShutdown(errors.WithMessage(err, "failed to update manifests"))
	}
}

func updateManifests(ctx context.Context, resolver *images.Resolver, ingressController []string, nodeConfig *daemonconfig.Node, cfg cmds.Agent, prime bool) error {
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
	os.MkdirAll(manifestsDir, 0700)

	// TODO: Remove this once we no longer support detecting legacy installations of ingress-nginx

	// if ingress-controller is not set in config, determine what the default should be.
	// ingress-nginx is the default if its HelmChart is present in the cluster, otherwise traefik
	if len(ingressController) == 0 {
		logrus.Infof("Reading deployed HelmCharts to determine default ingress-controller...")
		defaultIngress, err := getDefaultIngressClassFromCharts(ctx, nodeConfig)
		if err != nil {
			return errors.WithMessage(err, "failed to determine default ingress-controller from deployed charts")
		}
		ingressController = []string{defaultIngress}
	}
	logrus.Infof("Using ingress-controller: %v", ingressController)

	tempChartsDir := refChartsDir + ".tmp"
	os.RemoveAll(tempChartsDir)

	// Copy bundled charts into a temp dir for rewriting before they are copied into place
	if err := copyDir(tempChartsDir, refChartsDir); err != nil {
		return errors.WithMessage(err, "failed to copy temporary charts")
	}
	defer os.RemoveAll(tempChartsDir)

	// Create empty base and crd AddOn for unselected ingress controllers
	// We can't disable it this late in startup, so we have to manually truncate them instead
	controllers := sets.New[string](ingressController...)
	for _, name := range cli.IngressItems {
		if !controllers.Has(name) {
			for _, f := range []string{"rke2-" + name + ".yaml", "rke2-" + name + "-crd.yaml"} {
				if f, err := os.OpenFile(filepath.Join(tempChartsDir, f), os.O_WRONLY|os.O_TRUNC, 0600); err == nil {
					f.Write([]byte("# disabled by configuration\n"))
					f.Close()
				}
			}
		}
	}

	// Fix up bundled charts in temp dir
	if err := setChartValues(tempChartsDir, ingressController[0], nodeConfig, cfg, prime); err != nil {
		return errors.WithMessage(err, "failed to rewrite bundled HelmChart manifests to pass through CLI values")
	}

	// Copy modified charts into the manifests directory, since the K3s
	// deploy controller will delete ones that are disabled
	if err := copyDir(manifestsDir, tempChartsDir); err != nil {
		return errors.WithMessage(err, "failed to copy runtime charts")
	}

	// Fix up user HelmCharts to pass through configured values
	if err := setChartValues(manifestsDir, ingressController[0], nodeConfig, cfg, prime); err != nil {
		logrus.Errorf("Failed to rewrite user HelmChart manifests to pass through CLI values: %v", err)
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

// copyDir recursively copies files from source to destination.
func copyDir(target, source string) error {
	return copy.Copy(source, target, copy.Options{NumOfWorkers: 0})
}

// setChartValues scans the directory at manifestDir. It attempts to load all manifests
// in that directory as HelmCharts. Any manifests that contain a HelmChart are modified to
// pass through settings to both the Helm job and the chart values.
func setChartValues(manifestsDir, ingressController string, nodeConfig *daemonconfig.Node, cfg cmds.Agent, prime bool) error {
	chartValues := map[string]string{
		"global.clusterCIDR":                  util.JoinIPNets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterCIDRv4":                util.JoinIP4Nets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterCIDRv6":                util.JoinIP6Nets(nodeConfig.AgentConfig.ClusterCIDRs),
		"global.clusterDNS":                   util.JoinIPs(nodeConfig.AgentConfig.ClusterDNSs),
		"global.clusterDomain":                nodeConfig.AgentConfig.ClusterDomain,
		"global.rke2DataDir":                  cfg.DataDir,
		"global.serviceCIDR":                  util.JoinIPNets(nodeConfig.AgentConfig.ServiceCIDRs),
		"global.systemDefaultIngressClass":    ingressController,
		"global.prime.enabled":                strconv.FormatBool(prime),
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
		return errors.WithMessagef(err, "failed to open manifest %s", fileName)
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
		logrus.Debugf("No cluster configuration value changes necessary for manifest %s", fileName)
		return nil
	}

	data, err := yaml.Export(objs...)
	if err != nil {
		return errors.WithMessagef(err, "failed to export modified manifest %s", fileName)
	}

	if _, err := fh.Seek(0, 0); err != nil {
		return errors.WithMessagef(err, "failed to seek in manifest %s", fileName)
	}

	if err := fh.Truncate(0); err != nil {
		return errors.WithMessagef(err, "failed to truncate manifest %s", fileName)
	}

	if _, err := fh.Write(data); err != nil {
		return errors.WithMessagef(err, "failed to write modified manifest %s", fileName)
	}

	if err := fh.Sync(); err != nil {
		return errors.WithMessagef(err, "failed to sync modified manifest %s", fileName)
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

// getDefaultIngressClassFromCharts gets the current default ingress class from installed chart values.
// If there are no charts present in the cluster, it returns the default.
func getDefaultIngressClassFromCharts(ctx context.Context, nodeConfig *daemonconfig.Node) (string, error) {
	hl, err := ListHelmCharts(ctx, nodeConfig.AgentConfig.KubeConfigK3sController)
	if err != nil {
		return "", err
	}
	for _, h := range hl.Items {
		switch h.Name {
		case "rke2-ingress-nginx", "rke2-traefik":
			if v, ok := h.Spec.Set["global.systemDefaultIngressClass"]; ok && v.Type == intstr.String {
				return v.StrVal, nil
			}
		}
	}
	return cli.IngressItems[0], nil
}
