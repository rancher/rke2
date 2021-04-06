package bootstrap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/pkg/errors"
	"github.com/rancher/k3s/pkg/agent/templates"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

// registry stores information necessary to configure authentication and
// connections to remote registries, including overriding registry endpoints
type registry struct {
	r *templates.Registry
	t map[string]*http.Transport
}

// Explicit interface checks
var _ authn.Keychain = &registry{}
var _ http.RoundTripper = &registry{}

// getPrivateRegistries loads private registry configuration from registries.yaml
func getPrivateRegistries(path string) (*registry, error) {
	registry := &registry{
		r: &templates.Registry{},
		t: map[string]*http.Transport{},
	}
	privRegistryFile, err := ioutil.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return registry, nil
		}
		return nil, err
	}
	logrus.Infof("Using registry config file at %s", path)
	if err := yaml.Unmarshal(privRegistryFile, registry.r); err != nil {
		return nil, err
	}
	return registry, nil
}

// Resolve returns an authenticator for the authn.Keychain interface. The authenticator
// provides credentials to a registry by looking up configuration from mirror endpoints.
func (r *registry) Resolve(target authn.Resource) (authn.Authenticator, error) {
	endpointURL, err := r.getEndpointForHost(target.RegistryStr())
	if err != nil {
		return nil, err
	}
	return r.getAuthenticatorForHost(endpointURL.Host)
}

// RoundTrip round-trips a HTTP request for the http.RoundTripper interface. The round-tripper
// overrides the Host in the headers and URL based on mirror endpoint configuration. It also
// configures TLS based on the endpoint's TLS config, if any.
func (r *registry) RoundTrip(req *http.Request) (*http.Response, error) {
	endpointURL, err := r.getEndpointForHost(req.URL.Host)
	if err != nil {
		return nil, err
	}

	// override request host and scheme
	req.Host = endpointURL.Host
	req.URL.Host = endpointURL.Host
	req.URL.Scheme = endpointURL.Scheme

	switch endpointURL.Scheme {
	case "http":
		return http.DefaultTransport.RoundTrip(req)
	case "https":
		// Create and cache transport if not found.
		if _, ok := r.t[endpointURL.Host]; !ok {
			tlsConfig, err := r.getTLSConfigForHost(endpointURL.Host)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to get TLS config for endpoint %s", endpointURL.Host)
			}

			r.t[endpointURL.Host] = &http.Transport{
				Proxy: http.ProxyFromEnvironment,
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				TLSClientConfig:       tlsConfig,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
			}
		}
		return r.t[endpointURL.Host].RoundTrip(req)
	}
	return nil, fmt.Errorf("unsupported scheme %s in registry endpoint", endpointURL.Scheme)
}

// getEndpointForHost gets endpoint configuration for a host. Because go-containerregistry's
// Keychain interface does not provide a good hook to check authentication for multiple endpoints,
// we only use the first endpoint from the mirror list. If no endpoint configuration is found, https
// is assumed.
func (r *registry) getEndpointForHost(host string) (*url.URL, error) {
	keys := []string{host}
	if host == name.DefaultRegistry {
		keys = append(keys, "docker.io")
	}
	keys = append(keys, "*")

	for _, key := range keys {
		if mirror, ok := r.r.Mirrors[key]; ok {
			endpointCount := len(mirror.Endpoints)
			switch {
			case endpointCount > 1:
				logrus.Warnf("Found more than one endpoint for %s; only the first entry will be used", host)
				fallthrough
			case endpointCount == 1:
				return url.Parse(mirror.Endpoints[0])
			}
		}
	}
	return url.Parse("https://" + host)
}

// getAuthenticatorForHost returns an Authenticator for a given host. This should be the host from
// the mirror endpoint list, NOT the host from the request. If no configuration is present,
// Anonymous authentication is used.
func (r *registry) getAuthenticatorForHost(host string) (authn.Authenticator, error) {
	if config, ok := r.r.Configs[host]; ok {
		if config.Auth != nil {
			return authn.FromConfig(authn.AuthConfig{
				Username:      config.Auth.Username,
				Password:      config.Auth.Password,
				Auth:          config.Auth.Auth,
				IdentityToken: config.Auth.IdentityToken,
			}), nil
		}
	}
	return authn.Anonymous, nil
}

// getTLSConfigForHost returns TLS configuration for a given host. This should be the host from the
// mirror endpoint list, NOT the host from the request.  This is cribbed from
// https://github.com/containerd/cri/blob/release/1.4/pkg/server/image_pull.go#L274
func (r *registry) getTLSConfigForHost(host string) (*tls.Config, error) {
	tlsConfig := &tls.Config{}
	if config, ok := r.r.Configs[host]; ok {
		if config.TLS != nil {
			if config.TLS.CertFile != "" && config.TLS.KeyFile == "" {
				return nil, errors.Errorf("cert file %q was specified, but no corresponding key file was specified", config.TLS.CertFile)
			}
			if config.TLS.CertFile == "" && config.TLS.KeyFile != "" {
				return nil, errors.Errorf("key file %q was specified, but no corresponding cert file was specified", config.TLS.KeyFile)
			}
			if config.TLS.CertFile != "" && config.TLS.KeyFile != "" {
				cert, err := tls.LoadX509KeyPair(config.TLS.CertFile, config.TLS.KeyFile)
				if err != nil {
					return nil, errors.Wrap(err, "failed to load cert file")
				}
				if len(cert.Certificate) != 0 {
					tlsConfig.Certificates = []tls.Certificate{cert}
				}
				tlsConfig.BuildNameToCertificate() // nolint:staticcheck
			}

			if config.TLS.CAFile != "" {
				caCertPool, err := x509.SystemCertPool()
				if err != nil {
					return nil, errors.Wrap(err, "failed to get system cert pool")
				}
				caCert, err := ioutil.ReadFile(config.TLS.CAFile)
				if err != nil {
					return nil, errors.Wrap(err, "failed to load CA file")
				}
				caCertPool.AppendCertsFromPEM(caCert)
				tlsConfig.RootCAs = caCertPool
			}

			tlsConfig.InsecureSkipVerify = config.TLS.InsecureSkipVerify
		}
	}

	return tlsConfig, nil
}
