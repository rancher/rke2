package images

import (
	"testing"
)

func TestResolver_RegistryOverrides(t *testing.T) {
	tests := []struct {
		name     string
		cfg      ImageOverrideConfig
		expected string
		ref      string
		wantErr  bool
	}{
		{
			name:     "default registry without override",
			cfg:      ImageOverrideConfig{},
			expected: "index.docker.io/rancher/rke2-runtime:latest",
			ref:      Runtime,
		},
		{
			name: "custom registry without prefix",
			cfg: ImageOverrideConfig{
				SystemDefaultRegistry: "example.com",
			},
			expected: "example.com/rancher/rke2-runtime:latest",
			ref:      Runtime,
		},
		{
			name: "registry with single level prefix",
			cfg: ImageOverrideConfig{
				SystemDefaultRegistry: "example.com/docker.io",
			},
			expected: "example.com/docker.io/rancher/rke2-runtime:latest",
			ref:      Runtime,
		},
		{
			name: "registry shortname without domain",
			cfg: ImageOverrideConfig{
				SystemDefaultRegistry: "foobar",
			},
			expected: "foobar/rancher/rke2-runtime:latest",
			ref:      Runtime,
		},
		{
			name: "registry with multi-level prefix",
			cfg: ImageOverrideConfig{
				SystemDefaultRegistry: "example.com/path/to/registry",
			},
			expected: "example.com/path/to/registry/rancher/rke2-runtime:latest",
			ref:      Runtime,
		},
		{
			name: "registry with port and prefix",
			cfg: ImageOverrideConfig{
				SystemDefaultRegistry: "example.com:5000/docker.io",
			},
			expected: "example.com:5000/docker.io/rancher/rke2-runtime:latest",
			ref:      Runtime,
		},
		{
			name: "registry with port but no prefix",
			cfg: ImageOverrideConfig{
				SystemDefaultRegistry: "example.com:5000",
			},
			expected: "example.com:5000/rancher/rke2-runtime:latest",
			ref:      Runtime,
		},
		{
			name: "registry with trailing slash",
			cfg: ImageOverrideConfig{
				SystemDefaultRegistry: "example.com/prefix/",
			},
			expected: "example.com/prefix/rancher/rke2-runtime:latest",
			ref:      Runtime,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver, err := NewResolver(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewResolver() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			ref, err := resolver.GetReference(tt.ref)
			if err != nil {
				t.Fatal(err)
			}

			if ref.Name() != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, ref.Name())
			}
		})
	}
}

func TestSplitRegistryAndPrefix(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		expectedHost   string
		expectedPrefix string
	}{
		{
			name:           "simple registry without prefix",
			input:          "example.com",
			expectedHost:   "example.com",
			expectedPrefix: "",
		},
		{
			name:           "registry with single level prefix",
			input:          "example.com/docker.io",
			expectedHost:   "example.com",
			expectedPrefix: "docker.io",
		},
		{
			name:           "registry with multi-level prefix",
			input:          "example.com/path/to/registry",
			expectedHost:   "example.com",
			expectedPrefix: "path/to/registry",
		},
		{
			name:           "registry with port and prefix",
			input:          "example.com:5000/myrepo",
			expectedHost:   "example.com:5000",
			expectedPrefix: "myrepo",
		},
		{
			name:           "empty string",
			input:          "",
			expectedHost:   "",
			expectedPrefix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, prefix := splitRegistryAndPrefix(tt.input)
			if host != tt.expectedHost {
				t.Errorf("host: expected %q, got %q", tt.expectedHost, host)
			}
			if prefix != tt.expectedPrefix {
				t.Errorf("prefix: expected %q, got %q", tt.expectedPrefix, prefix)
			}
		})
	}
}
