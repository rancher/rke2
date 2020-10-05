---
title: Agent Configuration Reference
---

You will see some options that can be passed in as both command flags and environment variables. For help with passing in options, refer to [How to Use Flags and Environment Variables.](../how_to_flags/how_to_flags.md)

### RKE2 Agent CLI Help

> If an option appears in brackets below, for example `[$RKE2_URL]`, it means that the option can be passed in as an environment variable of that name.

```bash
NAME:
   rke2 agent - Run node agent

USAGE:
   rke2 agent [OPTIONS]

OPTIONS:
   --config FILE, -c FILE              (config) Load configuration from FILE (default: "/etc/rancher/rke2/config.yaml") [$RKE2_CONFIG_FILE]
   --debug                             (logging) Turn on debug logs [$RKE2_DEBUG]
   --token value, -t value             (cluster) Token to use for authentication [$RKE2_TOKEN]
   --token-file value                  (cluster) Token file to use for authentication [$RKE2_TOKEN_FILE]
   --server value, -s value            (cluster) Server to connect to [$RKE2_URL]
   --data-dir value, -d value          (data) Folder to hold state (default: "/var/lib/rancher/rke2")
   --node-name value                   (agent/node) Node name [$RKE2_NODE_NAME]
   --node-label value                  (agent/node) Registering and starting kubelet with set of labels
   --node-taint value                  (agent/node) Registering kubelet with set of taints
   --container-runtime-endpoint value  (agent/runtime) Disable embedded containerd and use alternative CRI implementation
   --snapshotter value                 (agent/runtime) Override default containerd snapshotter (default: "overlayfs")
   --private-registry value            (agent/runtime) Private registry configuration file (default: "/etc/rancher/rke2/registries.yaml")
   --node-ip value, -i value           (agent/networking) IP address to advertise for node
   --resolv-conf value                 (agent/networking) Kubelet resolv.conf file [$RKE2_RESOLV_CONF]
   --kubelet-arg value                 (agent/flags) Customized flag for kubelet process
   --kube-proxy-arg value              (agent/flags) Customized flag for kube-proxy process
   --protect-kernel-defaults           (agent/node) Kernel tuning behavior. If set, error if kernel tunables are different than kubelet defaults.
   --selinux                           (agent/node) Enable SELinux in containerd [$RKE2_SELINUX]
   --system-default-registry value     (image) Private registry to be used for all system Docker images [$RKE2_SYSTEM_DEFAULT_REGISTRY]
   --cloud-provider-name value         (cloud provider) Cloud provider name [$RKE2_CLOUD_PROVIDER_NAME]
   --cloud-provider-config value       (cloud provider) Cloud provider configuration file path [$RKE2_CLOUD_PROVIDER_CONFIG]
   --profile value                     (security) Validate system configuration against the selected benchmark (valid items: cis-1.5) [$RKE2_CIS_PROFILE]
```
