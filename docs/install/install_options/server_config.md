---
title: Server Configuration Reference
---

This is a reference to all parameters that can be used to configure the rke2 server. Note that while this is a reference to the command line arguments, the best way to configure RKE2 is using the [configuration file](install_options.md#configuration-file).

### RKE2 Server CLI Help

> If an option appears in brackets below, for example `[$RKE2_TOKEN]`, it means that the option can be passed in as an environment variable of that name.

```bash
NAME:
   rke2 server - Run management server

USAGE:
   rke2 server [OPTIONS]

OPTIONS:
   --config FILE, -c FILE                 (config) Load configuration from FILE (default: "/etc/rancher/rke2/config.yaml") [$RKE2_CONFIG_FILE]
   --debug                                (logging) Turn on debug logs [$RKE2_DEBUG]
   --bind-address value                   (listener) rke2 bind address (default: 0.0.0.0)
   --advertise-address value              (listener) IP address that apiserver uses to advertise to members of the cluster (default: node-external-ip/node-ip)
   --tls-san value                        (listener) Add additional hostname or IP as a Subject Alternative Name in the TLS cert
   --data-dir value, -d value             (data) Folder to hold state (default: "/var/lib/rancher/rke2")
   --cluster-cidr value                   (networking) Network CIDR to use for pod IPs (default: "10.42.0.0/16")
   --service-cidr value                   (networking) Network CIDR to use for services IPs (default: "10.43.0.0/16")
   --cluster-dns value                    (networking) Cluster IP for coredns service. Should be in your service-cidr range (default: 10.43.0.10)
   --cluster-domain value                 (networking) Cluster Domain (default: "cluster.local")
   --token value, -t value                (cluster) Shared secret used to join a server or agent to a cluster [$RKE2_TOKEN]
   --token-file value                     (cluster) File containing the cluster-secret/token [$RKE2_TOKEN_FILE]
   --write-kubeconfig value, -o value     (client) Write kubeconfig for admin client to this file [$RKE2_KUBECONFIG_OUTPUT]
   --write-kubeconfig-mode value          (client) Write kubeconfig with this mode [$RKE2_KUBECONFIG_MODE]
   --kube-apiserver-arg value             (flags) Customized flag for kube-apiserver process
   --kube-scheduler-arg value             (flags) Customized flag for kube-scheduler process
   --kube-controller-manager-arg value    (flags) Customized flag for kube-controller-manager process
   --etcd-disable-snapshots               (db) Disable automatic etcd snapshots
   --etcd-snapshot-schedule-cron value    (db) Snapshot interval time in cron spec. eg. every 5 hours '* */5 * * *' (default: "0 */12 * * *")
   --etcd-snapshot-retention value        (db) Number of snapshots to retain (default: 5)
   --etcd-snapshot-dir value              (db) Directory to save db snapshots. (Default location: ${data-dir}/db/snapshots)
   --disable value                        (components) Do not deploy packaged components and delete any deployed components (valid items: rke2-canal, rke2-coredns, rke2-ingress, rke2-kube-proxy, rke2-metrics-server)
   --node-name value                      (agent/node) Node name [$RKE2_NODE_NAME]
   --node-label value                     (agent/node) Registering and starting kubelet with set of labels
   --node-taint value                     (agent/node) Registering kubelet with set of taints
   --container-runtime-endpoint value     (agent/runtime) Disable embedded containerd and use alternative CRI implementation
   --snapshotter value                    (agent/runtime) Override default containerd snapshotter (default: "overlayfs")
   --private-registry value               (agent/runtime) Private registry configuration file (default: "/etc/rancher/rke2/registries.yaml")
   --node-ip value, -i value              (agent/networking) IP address to advertise for node
   --node-external-ip value               (agent/networking) External IP address to advertise for node
   --resolv-conf value                    (agent/networking) Kubelet resolv.conf file [$RKE2_RESOLV_CONF]
   --kubelet-arg value                    (agent/flags) Customized flag for kubelet process
   --kube-proxy-arg value                 (agent/flags) Customized flag for kube-proxy process
   --protect-kernel-defaults              (agent/node) Kernel tuning behavior. If set, error if kernel tunables are different than kubelet defaults.
   --agent-token value                    (experimental/cluster) Shared secret used to join agents to the cluster, but not servers [$RKE2_AGENT_TOKEN]
   --agent-token-file value               (experimental/cluster) File containing the agent secret [$RKE2_AGENT_TOKEN_FILE]
   --server value, -s value               (experimental/cluster) Server to connect to, used to join a cluster [$RKE2_URL]
   --cluster-reset                        (experimental/cluster) Forget all peers and become sole member of a new cluster [$RKE2_CLUSTER_RESET]
   --cluster-reset-restore-path value     (db) Path to snapshot file to be restored
   --secrets-encryption                   (experimental) Enable Secret encryption at rest
   --selinux                              (agent/node) Enable SELinux in containerd [$RKE2_SELINUX]
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
   --profile value                        (security) Validate system configuration against the selected benchmark (valid items: cis-1.5) [$RKE2_CIS_PROFILE]
   --audit-policy-file value              (security) Path to the file that defines the audit policy configuration [$RKE2_AUDIT_POLICY_FILE]
```
