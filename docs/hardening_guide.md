# CIS Hardening Guide

This document provides prescriptive guidance for hardening a production installation of RKE2. It outlines the configurations and controls required to address Kubernetes benchmark controls from the Center for Information Security (CIS).

For more detail about evaluating a hardened cluster against the official CIS benchmark, refer to the [CIS Benchmark Rancher Self-Assessment Guide](cis_self_assessment.md).

RKE2 is designed to be "hardened by default" and pass the majority of the Kubernetes CIS controls without modification. There are a few notable exceptions to this that require manual intervention to fully pass the CIS Benchmark:

1. RKE2 will not modify the host operating system. Therefore, you, the operator, must make a few Host-level modifications.
2. Certain CIS policy controls for PodSecurityPolicies and NetworkPolicies will restrict the functionality of this cluster. You must opt into having RKE2 configuring these out of the box.

To help ensure these above requirements are met, RKE2 can be started with the `profile` parameter set to `cis-1.5`. This parameter generally does two things:

1. Checks that Host-level requirements have been met. If they haven't RKE2 will log a fatal error describing the requirement that has not been met and exit.
2. Configuring runtime policies PodSecurityPolicies and NetworkPolicies that allow the cluster to pass associated controls.

> **Note:** The profile's flag only valid value is `cis-1.5`. It accepts a string value to allow for other profiles in the future.

The following secction outlines the specific actions that are taken when the `profile` flag is set to `cis-1.5`.

## Host-level Requirements

There are two areas of of Host-level requirements: kernel parameters and etcd process/directory configuration. These are outlined in this section.

### Ensure `protect-kernel-defaults` is set
This is a kubelet flag that will cause the kubelet to exit if the required kernel parameters are unset or are set to values that are different from the kubelet's defaults.

When the `profile` parameter is set, RKE2 will set the flag to true. 

> **Note:** `protect-kernel-defaults` is exposed a top-level parameter for RKE2. If you have set `profile` to "cis-1.5" and `protect-kernel-defaults` to false explicitly, RKE2 where exit with an error.

RKE2 will also check the same kernel parameters that the kubelet does and exit with an error following the same rules as the kubelet. This is done as a convenience to help the operator more quickly and easily identify what kernel parameters are violationg the kubelet defaults.

### Ensure etcd is started properly
The CIS Benchmark requires that the etcd data directory be owned by the `etcd` user and group. This implicitly requires the etcd process to be ran as the host-level `etcd` user. To acheive this, RKE2 takes several steps when started with the cis-1.5 profile:

1. Check that the `etcd` user and group exists on the host. If they don't, exit with an error.
2. Create etcd's data directory with `etcd` as the user and group owner.
3. Ensure the etcd process is ran as the `etcd` user and group by setting the etcd static pod's SecurityContext approopriately.

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

#### Create the etcd user
```bash
useradd -r -c "etcd user" -s /sbin/nologin -M etcd
```

## Kubernetes Runtime Requirements

The runtime requirements to pass the CIS Benchmark are centered around pod security and network policies. These are outlined in this section.

### PodSecurityPolicies

RKE2 always runs with the PodSecurityPolicy admission controller turned on. However, when it is **not** started with the cis-1.5 profile, RKE2 will put an unrestriced policy in place that allows Kubernetes to run as though the PodSecurityPolicy admission controller was not enabled.

When ran with the cis-1.5 profile, RKE2 will put a much more restrictive set of policies in place. These policies meet the requirements outlined in section 5.2 of the CIS Benchmark.

> **Note:** The Kubernetes control plane components and critical additions such as CNI, DNS, and Ingress are ran as pods in the `kube-system` namespace. Therefore, this namespace will have a policy that is less restrictive so that these components can run properly.
<!--
**TODO:** Add a separate doc on our default PSP behavior that explains how the defaults can be overriden by the operator and link here.
-->

### NetworkPolicies

When ran with the cis-1.5 profile, RKE2 will put NetworkPolicies in place that pass the CIS Benchmark for Kubernetes's built-in namespcaes. These namespaces are: `kube-system`, `kube-public`, `kube-node-lease`, and `default`.

The NetworkPolicy used will only allow pods within the same namespace to talk to each other. The notable exception to this is that it allows DNS requests to be resolved.

> **Note:** Operators must manage network policies as normal for additional namespaces that are created.
<!--
**TODO:** Add a separate doc on our NP behavior that explains how the defaults can be overriden by the operator and link here.
-->

## Known Issues
The following are controls that RKE2 currently does not pass. Each gap will be explained and whether it can be passed through manual operator intervention or if it will be addressed in a future release.

### Control 3.2.1
Ensure that a minimal audit policy is created (Scored)
<details>
<summary>Rationale</summary>
Logging is an important detective control for all systems, to detect potential unauthorised access.
</details>

RKE2 currently does not support configuring audit logging. This is a [known issue](https://github.com/rancher/rke2/issues/410) that will be addressed in an upcoming release.

### Control 5.1.5
Ensure that default service accounts are not actively used. (Scored)
<details>
<summary>Rationale</summary>
    
Kubernetes provides a default service account which is used by cluster workloads where no specific service account is assigned to the pod.

Where access to the Kubernetes API from a pod is required, a specific service account should be created for that pod, and rights granted to that service account.

The default service account should be configured such that it does not provide a service account token and does not have any explicit rights assignments.
</details>

The remediation for this is to update the `automountServiceAccountToken` field to `false` for the `default` service account in each namespace.

For `default` service accounts in the built-in namespaces (`kube-system`, `kube-public`, `kube-node-lease`, and `default`), RKE2 does not automatically do this. You can manually update this field on these service accounts to passs the control.


## Conclusion

If you have followed this guide, your RKE2 cluster will be configured to pass the CIS Kubernetes Benchmark. You can review our [CIS Benchmark Self-Assessment Guide](cis_self_assessment.md) to understand how we verified each of the benchmark and how you can do the same on your cluster.
