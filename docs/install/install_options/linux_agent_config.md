---
title: Agent Configuration Reference
---

This is a reference to all parameters that can be used to configure the rke2 agent. Note that while this is a reference to the command line arguments, the best way to configure RKE2 is using the [configuration file](install_options.md#configuration-file).

### RKE2 Agent CLI Help

> If an option appears in brackets below, for example `[$RKE2_URL]`, it means that the option can be passed in as an environment variable of that name.

```console
NAME:
   rke2 agent - Run node agent

USAGE:
   rke2 agent [OPTIONS]

OPTIONS:
   --config FILE, -c FILE                 (config) Load configuration from FILE (default: "/etc/rancher/rke2/config.yaml") [$RKE2_CONFIG_FILE]
   --debug                                (logging) Turn on debug logs [$RKE2_DEBUG]
   --token value, -t value                (cluster) Token to use for authentication [$RKE2_TOKEN]
   --token-file value                     (cluster) Token file to use for authentication [$RKE2_TOKEN_FILE]
   --server value, -s value               (cluster) Server to connect to [$RKE2_URL]
   --data-dir value, -d value             (data) Folder to hold state (default: "/var/lib/rancher/rke2")
   --node-name value                      (agent/node) Node name [$RKE2_NODE_NAME]
   --node-label value                     (agent/node) Registering and starting kubelet with set of labels
   --node-taint value                     (agent/node) Registering kubelet with set of taints
   --container-runtime-endpoint value     (agent/runtime) Disable embedded containerd and use alternative CRI implementation
   --snapshotter value                    (agent/runtime) Override default containerd snapshotter (default: "overlayfs")
   --private-registry value               (agent/runtime) Private registry configuration file (default: "/etc/rancher/rke2/registries.yaml")
   --node-ip value, -i value              (agent/networking) IPv4/IPv6 addresses to advertise for node
   --node-external-ip value               (agent/networking) IPv4/IPv6 external IP addresses to advertise for node
   --resolv-conf value                    (agent/networking) Kubelet resolv.conf file [$RKE2_RESOLV_CONF]
   --kubelet-arg value                    (agent/flags) Customized flag for kubelet process
   --protect-kernel-defaults              (agent/node) Kernel tuning behavior. If set, error if kernel tunables are different than kubelet defaults.
   --selinux                              (agent/node) Enable SELinux in containerd [$RKE2_SELINUX]
   --lb-server-port value                 (agent/node) Local port for supervisor client load-balancer. If the supervisor and apiserver are not colocated an additional port 1 less than this port will also be used for the apiserver client load-balancer. (default: 6444) [$RKE2_LB_SERVER_PORT]
   --system-default-registry value        (image) Private registry to be used for all system Docker images [$RKE2_SYSTEM_DEFAULT_REGISTRY]
   --kube-apiserver-image value           (image) Override image to use for kube-apiserver [$RKE2_KUBE_APISERVER_IMAGE]
   --kube-controller-manager-image value  (image) Override image to use for kube-controller-manager [$RKE2_KUBE_CONTROLLER_MANAGER_IMAGE]
   --kube-scheduler-image value           (image) Override image to use for kube-scheduler [$RKE2_KUBE_SCHEDULER_IMAGE]
   --pause-image value                    (image) Override image to use for pause [$RKE2_PAUSE_IMAGE]
   --runtime-image value                  (image) Override image to use for runtime binaries (containerd, kubectl, crictl, etc) [$RKE2_RUNTIME_IMAGE]
   --etcd-image value                     (image) Override image to use for etcd [$RKE2_ETCD_IMAGE]
   --kubelet-path value                   (experimental/agent) Override kubelet binary path [$RKE2_KUBELET_PATH]
   --cloud-provider-name value            (cloud provider) Cloud provider name [$RKE2_CLOUD_PROVIDER_NAME]
   --cloud-provider-config value          (cloud provider) Cloud provider configuration file path [$RKE2_CLOUD_PROVIDER_CONFIG]
   --profile value                        (security) Validate system configuration against the selected benchmark (valid items: cis-1.5, cis-1.6 ) [$RKE2_CIS_PROFILE]
   --audit-policy-file value              (security) Path to the file that defines the audit policy configuration [$RKE2_AUDIT_POLICY_FILE]
```
