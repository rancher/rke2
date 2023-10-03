package images

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

// Image defaults overridden by config passed in and ImageOverrideConfig below
const (
	Runtime                = "runtime-image"
	KubeAPIServer          = "kube-apiserver-image"
	KubeControllerManager  = "kube-controller-manager-image"
	KubeProxy              = "kube-proxy-image"
	KubeScheduler          = "kube-scheduler-image"
	ETCD                   = "etcd-image"
	Pause                  = "pause-image"
	CloudControllerManager = "cloud-controller-manager-image"
)

// These defaults are overridden at build time and do not need to be updated here
var (
	DefaultRegistry                    = name.DefaultRegistry
	DefaultEtcdImage                   = "rancher/hardened-etcd"
	DefaultKubernetesImage             = "rancher/hardened-kubernetes"
	DefaultPauseImage                  = "rancher/mirrored-pause"
	DefaultRuntimeImage                = "rancher/rke2-runtime"
	DefaultCloudControllerManagerImage = "rancher/rke2-cloud-provider"
)

// ResolverOpt is an option to modify image resolution behavior.
type ResolverOpt func(name.Reference) (name.Reference, error)

// Resolver provides functionality to resolve an RKE2 image name to a reference.
type Resolver struct {
	registry  name.Registry
	overrides map[string]name.Reference
}

// ImageOverrideConfig stores configuration from the CLI.
type ImageOverrideConfig struct {
	SystemDefaultRegistry  string
	KubeAPIServer          string
	KubeControllerManager  string
	KubeProxy              string
	KubeScheduler          string
	Pause                  string
	Runtime                string
	ETCD                   string
	CloudControllerManager string
}

// NewResolver creates a new image resolver, with options to modify the resolver behavior.
func NewResolver(c ImageOverrideConfig) (*Resolver, error) {
	registry, err := name.NewRegistry(DefaultRegistry)
	if err != nil {
		return nil, err
	}

	r := Resolver{
		registry:  registry,
		overrides: map[string]name.Reference{},
	}

	// Validate and set image overrides from config
	config := [...]struct {
		i string
		n string
	}{
		{ETCD, c.ETCD},
		{KubeAPIServer, c.KubeAPIServer},
		{KubeControllerManager, c.KubeControllerManager},
		{KubeProxy, c.KubeProxy},
		{KubeScheduler, c.KubeScheduler},
		{Pause, c.Pause},
		{Runtime, c.Runtime},
		{CloudControllerManager, c.CloudControllerManager},
	}
	for _, s := range config {
		if err := r.ParseAndSetOverride(s.i, s.n); err != nil {
			return nil, errors.Wrapf(err, "failed to parse %s", s.i)
		}
	}

	// validate and set system-default-registry from config
	if c.SystemDefaultRegistry != "" {
		registry, err := name.NewRegistry(c.SystemDefaultRegistry)
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse system-default-registry")
		}
		r.registry = registry
	}
	return &r, nil
}

// ParseAndSetDefaultRegistry updates the default registry, if it can be parsed
// as a valid Registry
func (r *Resolver) ParseAndSetDefaultRegistry(s string) error {
	registry, err := name.NewRegistry(s)
	if err != nil {
		return errors.Wrap(err, "failed to parse system-default-registry")
	}
	r.registry = registry
	return nil
}

// ParseAndSetOverride sets an image override from a string, if it can be parsed as
// a valid Reference.
func (r *Resolver) ParseAndSetOverride(i, n string) error {
	n = strings.TrimSpace(n)
	if n == "" {
		return nil
	}
	ref, err := name.ParseReference(n, name.WeakValidation)
	if err != nil {
		return err
	}
	r.overrides[i] = ref
	return nil
}

// SetOverride set an image override from a Reference. If the reference is nil,
// the override is cleared.
func (r *Resolver) SetOverride(i string, n name.Reference) {
	if n == nil {
		delete(r.overrides, i)
	} else {
		r.overrides[i] = n
	}
}

// GetReference returns a reference to an image. If an override is set it is used,
// otherwise the compile-time default is retrieved and default-registry settings applied.
// Options can be passed to modify the reference before it is returned.
func (r *Resolver) GetReference(i string, opts ...ResolverOpt) (name.Reference, error) {
	var ref name.Reference
	if o, ok := r.overrides[i]; ok {
		// Use override if set
		ref = o
	} else {
		// No override; get compile-time default
		d, err := getDefaultImage(i)
		if err != nil {
			return nil, err
		}
		ref = d

		// Apply registry override
		d, err = setRegistry(ref, r.registry)
		if err != nil {
			return nil, err
		}
		ref = d
	}

	// Apply additional options
	for _, o := range opts {
		r, err := o(ref)
		if err != nil {
			return nil, err
		}
		ref = r
	}
	return ref, nil
}

func (r *Resolver) MustGetReference(i string, opts ...ResolverOpt) name.Reference {
	ref, err := r.GetReference(i, opts...)
	if err != nil {
		logrus.Fatalf("Failed to get image reference for %s: %v", i, err)
	}
	return ref
}

// WithRegistry overrides the registry when resolving the reference to an image.
func WithRegistry(s string) ResolverOpt {
	return func(r name.Reference) (name.Reference, error) {
		registry, err := name.NewRegistry(s)
		if err != nil {
			return nil, err
		}
		s, err := setRegistry(r, registry)
		if err != nil {
			return nil, err
		}
		return s, nil
	}
}

// setRegistry sets the registry on an image reference. This is necessary
// because the Reference type doesn't expose the Registry field.
func setRegistry(ref name.Reference, registry name.Registry) (name.Reference, error) {
	if t, ok := ref.(name.Tag); ok {
		t.Registry = registry
		return t, nil
	} else if d, ok := ref.(name.Digest); ok {
		d.Registry = registry
		return d, nil
	}
	return ref, errors.Errorf("unhandled Reference type: %T", ref)
}

// getDefaultImage gets the compile-time default image for a given name.
func getDefaultImage(i string) (name.Reference, error) {
	var s string
	switch i {
	case ETCD:
		s = DefaultEtcdImage
	case Runtime:
		s = DefaultRuntimeImage
	case Pause:
		s = DefaultPauseImage
	case CloudControllerManager:
		s = DefaultCloudControllerManagerImage
	case KubeAPIServer, KubeControllerManager, KubeProxy, KubeScheduler:
		s = DefaultKubernetesImage
	default:
		return nil, fmt.Errorf("unknown image %s", i)
	}

	return name.ParseReference(s, name.WeakValidation)
}

// Pull checks for preloaded images in dir. If they are available, nothing is done.
// If they are not available, it adds the image to name.txt in dir.
// This is used to get K3s to pre-pull images for static pods.
func Pull(dir, name string, image name.Reference) error {
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
	return ioutil.WriteFile(dest, []byte(image.Name()+"\n"), 0644)
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
