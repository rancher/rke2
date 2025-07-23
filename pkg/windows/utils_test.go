//go:build windows
// +build windows

package windows

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Microsoft/hcsshim"
)

const (
	bareMetalPlatform = "bare-metal"
	ec2Platform       = "ec2"
	gcePlatform       = "gce"
	eksPlatform       = "eks"
	azurePlatform     = "aks"
)

type mockNetworkProvider struct {
	networks map[string]*hcsshim.HNSNetwork
}

func (m *mockNetworkProvider) GetHNSNetworkByName(name string) (*hcsshim.HNSNetwork, error) {
	if network, exists := m.networks[name]; exists {
		return network, nil
	}
	return nil, errors.New("network not found")
}

// mockRoundTripper is a mock to that holds the responses and errors for a given URL.
type mockRoundTripper struct {
	responses map[string]*http.Response
	errors    map[string]error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	url := req.URL.String()
	if err, ok := m.errors[url]; ok {
		return nil, err
	}

	if resp, ok := m.responses[url]; ok {
		return resp, nil
	}

	// default to 404 for unmatched URLs.
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

func Test_UnitHasTimedOut(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"Request succeed- nil error", nil, false},
		{"Request failed- url.Error with timeout", makeURLError(true), true},
		{"Request failed- url.Error without timeout", makeURLError(false), false},
		{"Request failed- net.OpError with timeout", makeOpError(true), true},
		{"Request failed- net.OpError without timeout", makeOpError(false), false},
		{"Request failed- net.Error timeoutError", context.DeadlineExceeded, true},
		{"Request failed- generic error", errors.New("maybe other problem"), false},
		{"Request failed- closed network", errors.New("use of closed network connection"), true},
		{"Request failed- contain closed connection", fmt.Errorf("got %q", "use of closed network connection"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := hasTimedOut(tt.err)
			if got != tt.want {
				t.Errorf("hasTimedOut(%#v) = %v; want %v", tt.err, got, tt.want)
			}
		})
	}
}

func Test_UnitPlatformType(t *testing.T) {
	originalNetProvider := netProvider
	originalHTTPClient := httpClient
	defer func() {
		netProvider = originalNetProvider
		httpClient = originalHTTPClient
	}()

	tests := []struct {
		name        string
		setupMocks  func()
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name: "Success- AKS platform detected by HNS - Should detect AKS when azure network exists",
			setupMocks: func() {
				netProvider = &mockNetworkProvider{
					networks: map[string]*hcsshim.HNSNetwork{
						"azure": {Name: "azure"},
					},
				}
				httpClient = &http.Client{
					Transport: &mockRoundTripper{
						responses: make(map[string]*http.Response),
						errors:    nil,
					},
				}
			},
			want:    azurePlatform,
			wantErr: false,
		},
		{
			name: "Success- EKS platform detected by HNS - Should detect EKS when vpcbr network exists",
			setupMocks: func() {
				netProvider = &mockNetworkProvider{
					networks: map[string]*hcsshim.HNSNetwork{
						"vpcbr*": {Name: "vpcbr"},
					},
				}
				httpClient = &http.Client{
					Transport: &mockRoundTripper{
						responses: make(map[string]*http.Response),
						errors:    nil,
					},
				}
			},
			want:    eksPlatform,
			wantErr: false,
		},
		{
			name: "Success- EC2 platform detected metadata- Should detect EC2 metadata",
			setupMocks: func() {
				netProvider = &mockNetworkProvider{
					networks: make(map[string]*hcsshim.HNSNetwork),
				}
				httpClient = &http.Client{
					Transport: &mockRoundTripper{
						responses: map[string]*http.Response{
							"http://169.254.169.254/latest/meta-data/local-hostname": {
								StatusCode: http.StatusOK,
								Body:       http.NoBody,
								Header:     make(http.Header),
							},
						},
						errors: nil,
					},
				}
			},
			want:    ec2Platform,
			wantErr: false,
		},
		{
			name: "Success- GCE platform detected metadata - Should detect GCE metadata",
			setupMocks: func() {
				netProvider = &mockNetworkProvider{
					networks: make(map[string]*hcsshim.HNSNetwork),
				}
				httpClient = &http.Client{
					Transport: &mockRoundTripper{
						responses: map[string]*http.Response{
							"http://169.254.169.254/latest/meta-data/local-hostname": {
								StatusCode: http.StatusNotFound,
								Body:       http.NoBody,
								Header:     make(http.Header),
							},
							"http://metadata.google.internal/computeMetadata/v1/instance/hostname": {
								StatusCode: http.StatusOK,
								Body:       http.NoBody,
								Header:     make(http.Header),
							},
						},
						errors: nil,
					},
				}
			},
			want:    gcePlatform,
			wantErr: false,
		},
		{
			name: "Error: EC2 timeout fallback to bare-metal",
			setupMocks: func() {
				netProvider = &mockNetworkProvider{
					networks: make(map[string]*hcsshim.HNSNetwork),
				}
				httpClient = &http.Client{
					Transport: &mockRoundTripper{
						responses: nil,
						errors: map[string]error{
							"http://169.254.169.254/latest/meta-data/local-hostname": context.DeadlineExceeded,
						},
					},
				}
			},
			want:    bareMetalPlatform,
			wantErr: false,
		},
		{
			name: "Error: Default to bare-metal when all checks fail",
			setupMocks: func() {
				netProvider = &mockNetworkProvider{
					networks: make(map[string]*hcsshim.HNSNetwork),
				}
				httpClient = &http.Client{
					Transport: &mockRoundTripper{
						responses: map[string]*http.Response{
							"http://169.254.169.254/latest/meta-data/local-hostname": {
								StatusCode: http.StatusNotFound,
								Body:       http.NoBody,
								Header:     make(http.Header),
							},
							"http://metadata.google.internal/computeMetadata/v1/instance/hostname": {
								StatusCode: http.StatusNotFound,
								Body:       http.NoBody,
								Header:     make(http.Header),
							},
						},
						errors: nil,
					},
				}
			},
			want: bareMetalPlatform,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMocks()

			got, err := platformType()
			if tt.wantErr {
				if err == nil {
					t.Errorf("platformType() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("platformType() error = %v, want error containing %v", err, tt.errContains)
				}
			} else if err != nil {
				t.Errorf("platformType() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("platformType() = got %v, want %v (%s)", got, tt.want, tt.name)
			}
		})
	}
}

// makeURLError is a helper to build a *url.Error containing either a timeoutError or a generic error.
func makeURLError(timeout bool) error {
	var err error
	if timeout {
		err = context.DeadlineExceeded
	} else {
		err = errors.New("connection refused")
	}

	return &url.Error{
		Op:  "Get",
		URL: "http://example.com",
		Err: err,
	}
}

// makeOpError is a helper to build a *net.OpError containing either a timeoutError or a generic error.
func makeOpError(timeout bool) error {
	var err error
	if timeout {
		err = context.DeadlineExceeded
	} else {
		err = errors.New("connection refused")
	}

	return &net.OpError{
		Op:   "dial",
		Net:  "tcp",
		Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80},
		Err:  err,
	}
}
