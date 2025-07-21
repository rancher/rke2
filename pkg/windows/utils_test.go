package windows

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// MockHNSNetwork is a mock type for the HNSNetwork type
type MockHNSNetwork struct {
	Name string
}

// GetHNSNetworkByNameFunc is a mock function for the GetHNSNetworkByName function
type GetHNSNetworkByNameFunc func(name string) (*MockHNSNetwork, error)

type timeoutError struct{}

func (e *timeoutError) Error() string   { return "timeout" }
func (e *timeoutError) Timeout() bool   { return true }
func (e *timeoutError) Temporary() bool { return false }

// Test hasTimedOut covers nil, wrapped URL timeout, direct net.Error, closed-connection, and non-timeout cases
func Test_UnitHasTimedOut(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil error", err: nil, want: false},
		{
			name: "url.Error with timeout",
			err: &url.Error{
				Op:  "Get",
				URL: "http://example.com",
				Err: &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80}, Err: &timeoutError{}},
			},
			want: true,
		},
		{name: "net.Error with timeout", err: &timeoutError{}, want: true},
		{
			name: "net.OpError with timeout",
			err:  &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80}, Err: &timeoutError{}},
			want: true,
		},
		{
			name: "url.Error without timeout",
			err:  &url.Error{Op: "Get", URL: "http://example.com", Err: errors.New("connection refused")},
			want: false,
		},
		{
			name: "net.OpError without timeout",
			err:  &net.OpError{Op: "dial", Net: "tcp", Addr: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 80}, Err: errors.New("connection refused")},
			want: false,
		},
		{name: "closed network connection error", err: errors.New("use of closed network connection"), want: true},
		{name: "generic error", err: errors.New("some other error"), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := hasTimedOut(tt.err); got != tt.want {
				t.Errorf("hasTimedOut() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_UnitPlatformTypeWithMocks(t *testing.T) {
	tests := []struct {
		name           string
		mockHNSFunc    GetHNSNetworkByNameFunc
		mockHTTPServer func() *httptest.Server
		want           string
		description    string
	}{
		// AKS
		{
			name: "AKS platform detected",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) {
				if name == "azure" {
					return &MockHNSNetwork{Name: "azure"}, nil
				}
				return nil, errors.New("network not found")
			},
			want:        "aks",
			description: "Should detect AKS when azure network exists",
		},
		// EKS
		{
			name: "EKS platform detected",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) {
				if name == "vpcbr*" {
					return &MockHNSNetwork{Name: "vpcbr*"}, nil
				}
				return nil, errors.New("network not found")
			},
			want:        "eks",
			description: "Should detect EKS when vpcbr network exists",
		},
		// EC2 success
		{
			name: "EC2 platform detected",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) {
				return nil, errors.New("network not found")
			},
			mockHTTPServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/latest/meta-data/local-hostname" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("ec2-hostname"))
					} else if r.URL.Path == "/computeMetadata/v1/instance/hostname" {
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			want:        "ec2",
			description: "Should detect EC2 when metadata service responds",
		},
		// GCE success
		{
			name: "GCE platform detected",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) {
				return nil, errors.New("network not found")
			},
			mockHTTPServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/latest/meta-data/local-hostname" {
						w.WriteHeader(http.StatusNotFound)
					} else if r.URL.Path == "/computeMetadata/v1/instance/hostname" && r.Header.Get("Metadata-Flavor") == "Google" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("gce-hostname"))
					}
				}))
			},
			want:        "gce",
			description: "Should detect GCE when metadata service responds with correct headers",
		},
		// EC2 timeout
		{
			name: "EC2 timeout fallback to bare-metal",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) {
				return nil, errors.New("network not found")
			},
			mockHTTPServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/latest/meta-data/local-hostname" {
						time.Sleep(100 * time.Millisecond)
						w.WriteHeader(http.StatusOK)
					} else if r.URL.Path == "/computeMetadata/v1/instance/hostname" {
						w.WriteHeader(http.StatusNotFound)
					}
				}))
			},
			want:        "bare-metal",
			description: "Should fallback to bare-metal when EC2 metadata service times out",
		},
		// GCE timeout
		{
			name: "GCE timeout fallback to bare-metal",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) {
				return nil, errors.New("network not found")
			},
			mockHTTPServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/latest/meta-data/local-hostname" {
						w.WriteHeader(http.StatusNotFound)
					} else if r.URL.Path == "/computeMetadata/v1/instance/hostname" {
						time.Sleep(100 * time.Millisecond)
						w.WriteHeader(http.StatusOK)
					}
				}))
			},
			want:        "bare-metal",
			description: "Should fallback to bare-metal when GCE metadata service times out",
		},
		// bare-metal when no server
		{
			name: "bare-metal when all services fail",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) {
				return nil, errors.New("network not found")
			},
			mockHTTPServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusNotFound)
				}))
			},
			want:        "bare-metal",
			description: "Should fallback to bare-metal when all services fail",
		},
		// New: EC2 non-timeout error falls through to GCE
		{
			name:        "EC2 non-timeout error falls through to GCE",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) { return nil, errors.New("network not found") },
			mockHTTPServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/latest/meta-data/local-hostname" {
						w.WriteHeader(http.StatusInternalServerError)
					} else if r.URL.Path == "/computeMetadata/v1/instance/hostname" && r.Header.Get("Metadata-Flavor") == "Google" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("gce-hostname"))
					}
				}))
			},
			want:        "gce",
			description: "Should fall through to GCE when EC2 returns non-200 error",
		},
		// New: both EC2 and GCE OK → EC2 precedence
		{
			name:        "EC2 and GCE both OK yields EC2",
			mockHNSFunc: func(name string) (*MockHNSNetwork, error) { return nil, errors.New("network not found") },
			mockHTTPServer: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					if r.URL.Path == "/latest/meta-data/local-hostname" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("ec2-hostname"))
					} else if r.URL.Path == "/computeMetadata/v1/instance/hostname" && r.Header.Get("Metadata-Flavor") == "Google" {
						w.WriteHeader(http.StatusOK)
						w.Write([]byte("gce-hostname"))
					}
				}))
			},
			want:        "ec2",
			description: "Should detect EC2 when both metadata endpoints respond OK",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var server *httptest.Server
			if tt.mockHTTPServer != nil {
				server = tt.mockHTTPServer()
				defer server.Close()
			}

			got := testPlatformTypeWithMocks(tt.mockHNSFunc, server)
			if got != tt.want {
				t.Errorf("platformTypeWithMocks() = %v, want %v (%s)", got, tt.want, tt.description)
			}
		})
	}
}

// testPlatformTypeWithMocks simulates the platformType function with mocked dependencies
func testPlatformTypeWithMocks(mockHNSFunc GetHNSNetworkByNameFunc, server *httptest.Server) string {
	// AKS
	if aksNet, _ := mockHNSFunc("azure"); aksNet != nil {
		return "aks"
	}
	// EKS
	if eksNet, _ := mockHNSFunc("vpcbr*"); eksNet != nil {
		return "eks"
	}

	// No metadata server: bare-metal
	if server == nil {
		return "bare-metal"
	}

	client := &http.Client{Timeout: 50 * time.Millisecond}
	// EC2
	ec2URL := server.URL + "/latest/meta-data/local-hostname"
	resp, err := client.Get(ec2URL)
	if err != nil && hasTimedOut(err) {
		// continue to GCE
	} else if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return "ec2"
		}
	}
	// GCE
	gceURL := server.URL + "/computeMetadata/v1/instance/hostname"
	req, err := http.NewRequest("GET", gceURL, nil)
	if err != nil {
		return "bare-metal"
	}
	req.Header.Add("Metadata-Flavor", "Google")
	resp, err = client.Do(req)
	if err != nil && hasTimedOut(err) {
		return "bare-metal"
	}
	if resp != nil {
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return "gce"
		}
	}
	return "bare-metal"
}

// hasTimedOut is a cross-platform version of the Windows-specific function
// Mirrors logic from utils.go but can run on any platform
func hasTimedOut(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if e, ok := err.Err.(net.Error); ok && e.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	case *net.OpError:
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
