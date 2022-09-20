package rke2

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
)

const (
	defaultPSAConfigFile = "/etc/rancher/rke2/rke2-pss.yaml"
)

// setPSAs sets the default PSA's based on the mode that RKE2 is running in. There is either CIS or non
// CIS mode. For CIS mode, a default PSA configuration with enforcement for restricted will be applied
// for non CIS mode, a default PSA configuration will be applied that has privileged restriction
func setPSAs(cisMode bool) error {
	logrus.Info("Applying Pod Security Admission Configuration")
	configDir := filepath.Dir(defaultPSAConfigFile)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return err
	}
	if !cisMode { // non-CIS mode
		psaConfig := unrestrictedPSAConfig()
		if err := ioutil.WriteFile(defaultPSAConfigFile, []byte(psaConfig), 0600); err != nil {
			return errors.Wrapf(err, "psa: failed to write psa unrestricted config")
		}

	} else { // CIS mode
		psaConfig := restrictedPSAConfig()
		if err := ioutil.WriteFile(defaultPSAConfigFile, []byte(psaConfig), 0600); err != nil {
			return errors.Wrapf(err, "psa: failed to write psa restricted config")
		}
	}
	return nil
}

func restrictedPSAConfig() string {
	psRestrictedConfig := `apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: PodSecurity
  configuration:
    apiVersion: pod-security.admission.config.k8s.io/v1beta1
    kind: PodSecurityConfiguration
    defaults:
      enforce: "restricted"
      enforce-version: "latest"
      audit: "restricted"
      audit-version: "latest"
      warn: "restricted"
      warn-version: "latest"
    exemptions:
      usernames: []
      runtimeClasses: []
      namespaces: [kube-system, cis-operator-system, tigera-operator]`
	return psRestrictedConfig
}

func unrestrictedPSAConfig() string {
	psUnrestrictedConfig := `apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- name: PodSecurity
  configuration:
    apiVersion: pod-security.admission.config.k8s.io/v1beta1
    kind: PodSecurityConfiguration
    defaults:
      enforce: "privileged"
      enforce-version: "latest"
    exemptions:
      usernames: []
      runtimeClasses: []
      namespaces: []`
	return psUnrestrictedConfig
}
