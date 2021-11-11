# CIS Hardening Guide

This document provides prescriptive guidance for hardening a production installation of RKE2. It outlines the configurations and controls required to address Kubernetes benchmark controls from the Center for Information Security (CIS).

For more detail about evaluating a hardened cluster against the official CIS benchmark, refer to the CIS Benchmark Rancher Self-Assessment Guide [v1.5](cis_self_assessment15.md) or [v1.6](cis_self_assessment16.md).

RKE2 is designed to be "hardened by default" and pass the majority of the Kubernetes CIS controls without modification. There are a few notable exceptions to this that require manual intervention to fully pass the CIS Benchmark:

1. RKE2 will not modify the host operating system. Therefore, you, the operator, must make a few Host-level modifications.
2. Certain CIS policy controls for PodSecurityPolicies and NetworkPolicies will restrict the functionality of this cluster. You must opt into having RKE2 configuring these out of the box.

To help ensure these above requirements are met, RKE2 can be started with the `profile` flag set to `cis-1.5` or `cis-1.6`. This flag generally does two things:

1. Checks that host-level requirements have been met. If they haven't, RKE2 will exit with a fatal error describing the unmet requirements.
2. Configures runtime Pod Security Policies and Network Policies that allow the cluster to pass associated controls.

> **Note:** The profile's flag only valid values are `cis-1.5` or `cis-1.6`. It accepts a string value to allow for other profiles in the future.

The following section outlines the specific actions that are taken when the `profile` flag is set to `cis-1.5` or `cis-1.6`.

## Host-level Requirements

There are two areas of Host-level requirements: kernel parameters and etcd process/directory configuration. These are outlined in this section.

### Ensure `protect-kernel-defaults` is set
This is a kubelet flag that will cause the kubelet to exit if the required kernel parameters are unset or are set to values that are different from the kubelet's defaults.

When the `profile` flag is set, RKE2 will set the flag to true.

> **Note:** `protect-kernel-defaults` is exposed a top-level flag for RKE2. If you have set `profile` to "cis-1.x" and `protect-kernel-defaults` to false explicitly, RKE2 will exit with an error.

RKE2 will also check the same kernel parameters that the kubelet does and exit with an error following the same rules as the kubelet. This is done as a convenience to help the operator more quickly and easily identify what kernel parameters are violating the kubelet defaults.

### Ensure etcd is started properly
The CIS Benchmark requires that the etcd data directory be owned by the `etcd` user and group. This implicitly requires the etcd process to be ran as the host-level `etcd` user. To achieve this, RKE2 takes several steps when started with the cis-1.5 profile:

1. Check that the `etcd` user and group exists on the host. If they don't, exit with an error.
2. Create etcd's data directory with `etcd` as the user and group owner.
3. Ensure the etcd process is ran as the `etcd` user and group by setting the etcd static pod's SecurityContext appropriately.

### Setting up hosts
This section gives you the commands necessary to configure your host to meet the above requirements.

#### Set kernel parameters
When RKE2 is installed, it creates a sysctl config file to set the required parameters appropriately.
However, it does not automatically configure the Host to use this configuration. You must do this manually.
The location of the config file depends on the installation method used.

If RKE2 was installed via RPM, YUM, or DNF (the default on OSes that use RPMs, such as CentOS), run the following command(s):
```bash
sudo cp -f /usr/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
sudo systemctl restart systemd-sysctl
```

If RKE2 was installed via the tarball (the default on OSes that do not use RPMs, such as Ubuntu), run the following command:
```bash
sudo cp -f /usr/local/share/rke2/rke2-cis-sysctl.conf /etc/sysctl.d/60-rke2-cis.conf
sudo systemctl restart systemd-sysctl
```

If your system lacks the `systemd-sysctl.service` and/or the `/etc/sysctl.d` directory you will want to make sure the
sysctls are applied at boot by running the following command during start-up:
```bash
sysctl -p /usr/local/share/rke2/rke2-cis-sysctl.conf
```

Please perform this step only on fresh installations, before actually using RKE2 to deploy Kubernetes. Many
Kubernetes components, including CNI plugins, are setting up their own sysctls. Restarting the
`systemd-sysctl` service on a running Kubernetes cluster can result in unexpected side-effects.

#### Create the etcd user
On some Linux distributions, the `useradd` command will not create a group. The `-U` flag is included below to account for that. This flag tells `useradd` to create a group with the same name as the user.
 
```bash
useradd -r -c "etcd user" -s /sbin/nologin -M etcd -U
```

## Kubernetes Runtime Requirements

The runtime requirements to pass the CIS Benchmark are centered around pod security and network policies. These are outlined in this section.

### PodSecurityPolicies

RKE2 always runs with the PodSecurityPolicy admission controller turned on. However, when it is **not** started with the cis-1.5 profile, RKE2 will put an unrestricted policy in place that allows Kubernetes to run as though the PodSecurityPolicy admission controller was not enabled.

When ran with the cis-1.5 profile, RKE2 will put a much more restrictive set of policies in place. These policies meet the requirements outlined in section 5.2 of the CIS Benchmark.

> **Note:** The Kubernetes control plane components and critical additions such as CNI, DNS, and Ingress are ran as pods in the `kube-system` namespace. Therefore, this namespace will have a policy that is less restrictive so that these components can run properly.

### NetworkPolicies

When ran with the cis-1.5 profile, RKE2 will put NetworkPolicies in place that passes the CIS Benchmark for Kubernetes's built-in namespaces. These namespaces are: `kube-system`, `kube-public`, `kube-node-lease`, and `default`.

The NetworkPolicy used will only allow pods within the same namespace to talk to each other. The notable exception to this is that it allows DNS requests to be resolved.

> **Note:** Operators must manage network policies as normal for additional namespaces that are created.

## Known Issues
The following are controls that RKE2 currently does not pass. Each gap will be explained and whether it can be passed through manual operator intervention or if it will be addressed in a future release.

## Conclusion

If you have followed this guide, your RKE2 cluster will be configured to pass the CIS Kubernetes Benchmark. You can review our CIS Benchmark Self-Assessment Guide [v1.5](cis_self_assessment15.md) or [v1.6](cis_self_assessment16.md) to understand how we verified each of the benchmarks and how you can do the same on your cluster.
