package windows

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

const (
	bareMetalPlatform = "bare-metal"
	ec2Platform       = "ec2"
	gcePlatform       = "gce"
	eksPlatform       = "eks"
	azurePlatform     = "aks"
)

// mockHNSNetwork is a mock to simulate getting HNS networks by name.
type mockHNSNetwork struct {
	name string
}

// mockConfig holds the mock functions and responses for the platform detection logic.
type mockConfig struct {
	hnsFunc        func(name string) (*mockHNSNetwork, error)
	ec2Response    *httptest.ResponseRecorder
	ec2Timeout     bool
	gceResponse    *httptest.ResponseRecorder
	gceTimeout     bool
	gceRequestFail bool
}

var (
	mockMu sync.RWMutex
	mock   *mockConfig
)

func resetMocks() {
	mockMu.Lock()
	defer mockMu.Unlock()

	mock = &mockConfig{
		hnsFunc: func(name string) (*mockHNSNetwork, error) {
			return nil, errors.New("network not found")
		},
	}
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

func Test_UnitPlatformTypeWithMocks(t *testing.T) {
	tests := []struct {
		name        string
		setupMocks  mockConfig
		want        string
		wantErr     bool
		errContains string
	}{
		{
			name: "Success- AKS platform detected by HNS - Should detect AKS when azure network exists",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					if name == "azure" {
						return &mockHNSNetwork{name: "azure"}, nil
					}
					return nil, errors.New("network not found")
				},
			},
			want:    azurePlatform,
			wantErr: false,
		},
		{
			name: "Success- EKS platform detected by HNS - Should detect EKS when vpcbr network exists",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					if name == "vpcbr*" {
						return &mockHNSNetwork{name: "vpcbr"}, nil
					}
					return nil, errors.New("network not found")
				},
			},
			want:    eksPlatform,
			wantErr: false,
		},
		{
			name: "Success- EC2 platform detected metadata- Should detect EC2 metadata",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					return nil, errors.New("network not found")
				},
				ec2Response: createResponse(200, "ec2-hostname"),
				gceResponse: createResponse(400, ""),
			},
			want:    ec2Platform,
			wantErr: false,
		},
		{
			name: "Success- GCE platform detected metadata - Should detect GCE metadata",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					return nil, errors.New("network not found")
				},
				ec2Response: createResponse(404, ""),
				gceResponse: createResponse(200, "gce-hostname"),
			},
			want:    gcePlatform,
			wantErr: false,
		},
		{
			name: "Error: EC2 timeout fallback to bare-metal - Should return bare-metal when EC2 metadata request times out",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					return nil, errors.New("network not found")
				},
				ec2Timeout:  true,
				gceResponse: createResponse(404, ""),
			},
			want:    bareMetalPlatform,
			wantErr: false,
		},
		{
			name: "Error: GCE timeout fallback to bare-metal - Should return bare-metal when GCE metadata request times out",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					return nil, errors.New("network not found")
				},
				ec2Response: createResponse(404, ""),
				gceTimeout:  true,
			},
			want:    bareMetalPlatform,
			wantErr: false,
		},
		{
			name: "Error: All services timeout and  default to bare-metal",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					return nil, errors.New("network not found")
				},
				ec2Timeout: true,
				gceTimeout: true,
			},
			want:    bareMetalPlatform,
			wantErr: false,
		},
		{
			name: "Error: GCE GET request fails not due to timeout",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					return nil, errors.New("network not found")
				},
				ec2Response: createResponse(404, ""),
				// Simulate GCE request failure.
				gceResponse:    nil,
				gceTimeout:     false,
				gceRequestFail: true,
			},
			want:        "",
			wantErr:     true,
			errContains: "oops something went wrong with GCE request",
		},
		{
			name: "Error: No services configured returns bare-metal",
			setupMocks: mockConfig{
				hnsFunc: func(name string) (*mockHNSNetwork, error) {
					return nil, errors.New("network not found")
				},
				ec2Response: nil,
				ec2Timeout:  false,
				gceResponse: nil,
				gceTimeout:  false,
			},
			want: bareMetalPlatform,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetMocks()
			mock = &tt.setupMocks

			got, err := mockPlatformType()
			if tt.wantErr {
				if err == nil {
					t.Errorf("mockPlatformType() expected error but got none")
					return
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("mockPlatformType() error = %v, want error containing %v", err, tt.errContains)
				}
			} else if err != nil {
				t.Errorf("mockPlatformType() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("platformTypeWithMocks() = %v, want %v (%s)", got, tt.want, tt.name)
			}
		})
	}
}

// https://github.com/rancher/rke2/blob/4ca66a5fc2eedc38a963f743618b99632fafdd6f/pkg/windows/utils.go#L154
// mockPlatformType is a mock implementation of platformType function.
func mockPlatformType() (string, error) {
	mockMu.RLock()
	defer mockMu.RUnlock()

	if aksNet, _ := mock.hnsFunc("azure"); aksNet != nil {
		return azurePlatform, nil
	}

	if eksNet, _ := mock.hnsFunc("vpcbr*"); eksNet != nil {
		return eksPlatform, nil
	}

	// EC2
	if mock.ec2Response != nil {
		resp := mock.ec2Response.Result()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return ec2Platform, nil
		}
	}
	if mock.ec2Timeout {
		return bareMetalPlatform, nil
	}

	// GCE
	// Simulate GCE request Failure.
	if mock.gceRequestFail {
		return "", errors.New("oops something went wrong with GCE request")
	}
	if mock.gceResponse != nil {
		resp := mock.gceResponse.Result()
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return gcePlatform, nil
		}
	}

	if mock.gceTimeout {
		return bareMetalPlatform, nil
	}

	return bareMetalPlatform, nil
}

func hasTimedOut(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if e, ok := err.Err.(net.Error); ok && e.Timeout() {
			return true
		}
	case *net.OpError:
		if err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	}
	errTxt := "use of closed network connection"
	if err != nil && strings.Contains(err.Error(), errTxt) {
		return true
	}

	return false
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

// createResponse is a helper function to create a *httptest.ResponseRecorder with a given status and body.
func createResponse(status int, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	rec.WriteHeader(status)
	if body != "" {
		rec.Write([]byte(body))
	}

	return rec
}
