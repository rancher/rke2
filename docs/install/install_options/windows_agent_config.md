---
title: Windows Agent Configuration Reference
---

This is a reference to all parameters that can be used to configure the Windows RKE2 agent.

### Windows RKE2 Agent CLI Help
**Windows Support is currently Experimental as of v1.21.3+rke2r1**

```console
NAME:
   rke2.exe agent - Run node agent

USAGE:
   rke2.exe agent command [command options] [arguments...]

COMMANDS:
   service  Manage RKE2 as a Windows Service

OPTIONS:
   --config FILE, -c FILE                     (config) Load configuration from FILE (default: "/etc/rancher/rke2/config.yaml") [%RKE2_CONFIG_FILE%]
   --debug                                    (logging) Turn on debug logs [%RKE2_DEBUG%]
   --token value, -t value                    (cluster) Token to use for authentication [%RKE2_TOKEN%]
   --token-file value                         (cluster) Token file to use for authentication [%RKE2_TOKEN_FILE%]
   --server value, -s value                   (cluster) Server to connect to [%RKE2_URL%]
   --data-dir value, -d value                 (data) Folder to hold state (default: "/var/lib/rancher/rke2")
   --node-name value                          (agent/node) Node name [%RKE2_NODE_NAME%]
   --node-label value                         (agent/node) Registering and starting kubelet with set of labels
   --node-taint value                         (agent/node) Registering kubelet with set of taints
   --image-credential-provider-bin-dir value  (agent/node) The path to the directory where credential provider plugin binaries are located (default: "/var/lib/rancher/credentialprovider/bin")
   --image-credential-provider-config value   (agent/node) The path to the credential provider plugin config file (default: "/var/lib/rancher/credentialprovider/config.yaml")
   --container-runtime-endpoint value         (agent/runtime) Disable embedded containerd and use alternative CRI implementation
   --snapshotter value                        (agent/runtime) Override default containerd snapshotter (default: "native")
   --private-registry value                   (agent/runtime) Private registry configuration file (default: "/etc/rancher/rke2/registries.yaml")
   --node-ip value, -i value                  (agent/networking) IPv4/IPv6 addresses to advertise for node
   --node-external-ip value                   (agent/networking) IPv4/IPv6 external IP addresses to advertise for node
   --resolv-conf value                        (agent/networking) Kubelet resolv.conf file [%RKE2_RESOLV_CONF%]
   --kubelet-arg value                        (agent/flags) Customized flag for kubelet process
   --kube-proxy-arg value                     (agent/flags) Customized flag for kube-proxy process
   --protect-kernel-defaults                  (agent/node) Kernel tuning behavior. If set, error if kernel tunables are different than kubelet defaults.
   --selinux                                  (agent/node) Enable SELinux in containerd [%RKE2_SELINUX%]
   --lb-server-port value                     (agent/node) Local port for supervisor client load-balancer. If the supervisor and apiserver are not colocated an additional port 1 less than this port will also be used for the apiserver client load-balancer. (default: 6444) [%RKE2_LB_SERVER_PORT%]
   --kube-apiserver-image value               (image) Override image to use for kube-apiserver [%RKE2_KUBE_APISERVER_IMAGE%]
   --kube-controller-manager-image value      (image) Override image to use for kube-controller-manager [%RKE2_KUBE_CONTROLLER_MANAGER_IMAGE%]
   --kube-proxy-image value                   (image) Override image to use for kube-proxy [%RKE2_KUBE_PROXY_IMAGE%]
   --kube-scheduler-image value               (image) Override image to use for kube-scheduler [%RKE2_KUBE_SCHEDULER_IMAGE%]
   --pause-image value                        (image) Override image to use for pause [%RKE2_PAUSE_IMAGE%]
   --runtime-image value                      (image) Override image to use for runtime binaries (containerd, kubectl, crictl, etc) [%RKE2_RUNTIME_IMAGE%]
   --etcd-image value                         (image) Override image to use for etcd [%RKE2_ETCD_IMAGE%]
   --kubelet-path value                       (experimental/agent) Override kubelet binary path [%RKE2_KUBELET_PATH%]
   --cloud-provider-name value                (cloud provider) Cloud provider name [%RKE2_CLOUD_PROVIDER_NAME%]
   --cloud-provider-config value              (cloud provider) Cloud provider configuration file path [%RKE2_CLOUD_PROVIDER_CONFIG%]
   --profile value                            (security) Validate system configuration against the selected benchmark (valid items: cis-1.5, cis-1.6 ) [%RKE2_CIS_PROFILE%]
   --audit-policy-file value                  (security) Path to the file that defines the audit policy configuration [%RKE2_AUDIT_POLICY_FILE%]
   --help, -h                                 show help
```
