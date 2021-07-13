// +build windows

package windows

import (
	"bytes"
	"html/template"
	"strings"
)

const calicoConfigTemplate = `{
  "name": "{{ .CalicoConfig.Name }}",
  "windows_use_single_network": true,
  "cniVersion": "{{ .CalicoConfig.CNI.Version }}",
  "type": "calico",
  "mode": "{{ .CalicoConfig.Mode }}",
  "vxlan_mac_prefix":  "{{ .CalicoConfig.Felix.MacPrefix }}",
  "vxlan_vni": {{ .CalicoConfig.Felix.Vxlanvni }},
  "policy": {
    "type": "k8s"
  },
  "log_level": "info",
  "capabilities": {"dns": true},
  "DNS": {
    "Nameservers": [
		"{{ .CalicoConfig.DNSServers }}"
    ],
    "Search":  [
      "svc.cluster.local"
    ]
  },
  "nodename_file": "{{ replace .CalicoConfig.NodeNameFile }}",
  "datastore_type": "{{ .CalicoConfig.DatastoreType }}",
  "etcd_endpoints": "{{ .CalicoConfig.ETCDEndpoints }}",
  "etcd_key_file": "{{ .CalicoConfig.ETCDKeyFile }}",
  "etcd_cert_file": "{{ .CalicoConfig.ETCDCertFile }}",
  "etcd_ca_cert_file": "{{ .CalicoConfig.ETCDCaCertFile }}",
  "kubernetes": {
    "kubeconfig": "{{ replace .CalicoConfig.KubeConfig.Path }}"
  },
  "ipam": {
    "type": "{{ .CalicoConfig.CNI.IpamType }}",
    "subnet": "usePodCidr"
  },
  "policies":  [
    {
      "Name":  "EndpointPolicy",
      "Value":  {
        "Type":  "OutBoundNAT",
        "ExceptionList":  [
          "{{ .CalicoConfig.ServiceCIDR }}"
        ]
      }
    },
    {
      "Name":  "EndpointPolicy",
      "Value":  {
        "Type":  "SDNROUTE",
        "DestinationPrefix":  "{{ .CalicoConfig.ServiceCIDR }}",
        "NeedEncap":  true
      }
    }
  ]
}
`

const calicoKubeConfigTemplate = `apiVersion: v1
kind: Config
clusters:
- name: kubernetes
  cluster:
    certificate-authority: {{ .CalicoConfig.KubeConfig.CertificateAuthority }}
    server: {{ .CalicoConfig.KubeConfig.Server }}
contexts:
- name: calico-windows@kubernetes
  context:
    cluster: kubernetes
    namespace: kube-system
    user: calico-windows
current-context: calico-windows@kubernetes
users:
- name: calico-windows
  user:
    token: {{ .CalicoConfig.KubeConfig.Token }}
`

// parseTemplateFromConfig takes a template and CNIConfig and generates a a windows specific
// CNI config.
func parseTemplateFromConfig(templateBuffer string, config *CNIConfig) (string, error) {
	out := &bytes.Buffer{}
	funcs := template.FuncMap{
		"replace": func(s string) string {
			return strings.ReplaceAll(s, "\\", "\\\\")
		},
	}
	t := template.Must(template.New("compiled_template").Funcs(funcs).Parse(templateBuffer))
	if err := t.Execute(out, config); err != nil {
		return "", err
	}
	return out.String(), nil
}
