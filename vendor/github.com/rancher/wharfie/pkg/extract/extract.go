package extract

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"strings"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

var (
	ErrIllegalPath = errors.New("illegal path")
	ps             = string(os.PathSeparator)
)

// An Option modifies the default file extraction behavior
type Option func(*options) error

type options struct {
	mode os.FileMode
}

// Extract extracts all content from the image to the provided path.
func Extract(img v1.Image, dir string, opts ...Option) error {
	dirs := map[string]string{ps: dir}
	return ExtractDirs(img, dirs, opts...)
}

// ExtractDirs extracts content from the image, honoring the directory map when
// deciding where on the local filesystem to place the extracted files. For example:
// {"/bin": "/usr/local/bin", "/etc": "/etc", "/etc/rancher": "/opt/rancher/etc"}
func ExtractDirs(img v1.Image, dirs map[string]string, opts ...Option) error {
	opt, err := makeOptions(opts...)
	if err != nil {
		return err
	}

	// Clean the directory map to ensure that source and destination reliably do
	// not have trailing slashes, unless the path is root. This is required to
	// make directory name matching reliable while walking up the source path.
	cleanDirs := make(map[string]string, len(dirs))
	for s, d := range dirs {
		var err error
		if s != ps {
			s = strings.TrimSuffix(s, ps)
		}
		if d != ps {
			d, err = filepath.Abs(strings.TrimSuffix(d, ps))
			if err != nil {
				return errors.Wrap(err, "invalid destination")
			}
		}
		cleanDirs[s] = d
	}

	reader := mutate.Extract(img)
	defer reader.Close()

	// Read from the tar until EOF
	t := tar.NewReader(reader)
	for {
		h, err := t.Next()
		if err == io.EOF {
			return nil
		} else if err != nil {
			return err
		}

		destination, err := findPath(cleanDirs, h.Name)
		if err != nil {
			return errors.Wrapf(err, "unable to extract file %s", h.Name)
		}
		if destination == "" {
			logrus.Debugf("Skipping file %s", h.Name)
			continue
		}

		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destination, opt.mode); err != nil {
				return err
			}
		case tar.TypeReg:
			logrus.Infof("Extracting file %s to %s", h.Name, destination)

			mode := h.FileInfo().Mode() & opt.mode
			f, err := os.OpenFile(destination, os.O_RDWR|os.O_CREATE|os.O_TRUNC, mode)
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
		case tar.TypeLink:
			target, err := findPath(cleanDirs, h.Linkname)
			if err != nil {
				return errors.Wrapf(err, "unable to create symlink %s", h.Name)
			}
			if target != "" {
				logrus.Infof("Creating symlink %s => %s", destination, target)
				_ = os.Remove(destination) // blind remove, if it fails the Symlink call will deal with it.
				err := os.Symlink(target, destination)
				if err != nil {
					return err
				}
			} else {
				logrus.Debugf("Skipping symlink %s", h.Name)
			}
		default:
			logrus.Warnf("Unhandled Typeflag %d for %s", h.Typeflag, h.Name)
		}
	}
}

// WithMode overrides the default mode used when extracting files and directories.
func WithMode(mode os.FileMode) Option {
	return func(o *options) error {
		o.mode = mode
		return nil
	}
}

// makeOptions applies Options, returning a modified option struct.
func makeOptions(opts ...Option) (*options, error) {
	o := &options{
		mode: 0755,
	}
	for _, option := range opts {
		if err := option(o); err != nil {
			return nil, err
		}
	}
	return o, nil
}

// findPath walks up the path, finding the longest match in the dirs map
func findPath(dirs map[string]string, path string) (string, error) {
	if !strings.HasPrefix(path, ps) {
		path = ps + path
	}

	for s := filepath.Dir(path); ; s = filepath.Dir(s) {
		if d, ok := dirs[s]; ok {
			j := filepath.Clean(filepath.Join(d, strings.TrimPrefix(path, s)))
			// Ensure that the path after cleaning does not escape the target prefix.
			if !strings.HasPrefix(j, d) {
				return "", ErrIllegalPath
			}
			return j, nil
		}
		if s == ps {
			return "", nil
		}
	}
}
