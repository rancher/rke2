---
title: Default Pod Security Standards
---
This document describes how RKE2 configures `PodSecurityStandards` and `NetworkPolicies` in order to be secure-by-default while also providing operators with maximum configuration flexibility.

!!! NOTE: This document applies to RKE2 versions equal to and after v1.25.0+rke2r1, if you want information for versions below v1.25.0+rke2r1 please refer to the [Default Policies Documentation](policies.md).

#### Pod Security Standards

Starting from Kubernetes version v1.25.0, PSPs are totally removed from Kubernetes, and replaced by Pod Security Admission. A default Pod SeRKE2 can be ran with or without the `profile: cis-1.23` configuration parameter. This will cause it to apply different security standards upon startup.curity Admission config file will be added to the cluster upon startup as follows:

* If running with the `cis-1.23` profile, RKE2 will apply a restricted pod security standard via a configuration file which will enforce `restricted` mode throughout the cluster with an exception to the `kube-system` and `cis-operator-system` namespaces to ensure successful operation of system pods.

* If running without the `cis-1.23` profile, RKE2 will apply a nonrestricted pod security standard via a configuration file which will enforce `privileged` mode throughout the cluster which allows a completely unrestricted mode to all pods in the cluster.

RKE2 will put this configuration file at `/etc/rancher/rke2/rke2-pss.yaml`, the content of the configuration file varries according to the cis mode which you started rke2:

**CIS Mode**

```yaml
apiVersion: apiserver.config.k8s.io/v1
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
      namespaces: [kube-system, cis-operator-system]
```

**Non CIS Mode**

```yaml
apiVersion: apiserver.config.k8s.io/v1
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
      namespaces: []
```

After placing this configuration file, rke2 will start the kube-apiserver with the following flag `--admission-control-config-file` which will be set to the path of the PSA config file.

If you want to override the default pod security standard configuration file, you can pass `pod-security-admission-config-file: <path-to-custom-psa-config-file>` to the RKE2 config file.

#### Network Policies

When RKE2 is run with the `profile: cis-1.23` parameter, it will apply 2 network policies to the `kube-system`, `kube-public`, and `default` namespaces and applies associated annotations. The same logic applies to these policies and annotations as the PSPs. On start, the annotations for each namespace are checked for existence and if they exist, RKE2 takes no action. If the annotation doesn't exist, RKE2 checks to see if the policy exists and if it does, recreates it.

The first policy applied is to restrict network traffic to only the namespace itself. See below.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  managedFields:
  - apiVersion: networking.k8s.io/v1
    fieldsType: FieldsV1
    fieldsV1:
      f:spec:
        f:ingress: {}
        f:policyTypes: {}
  name: default-network-policy
  namespace: default
spec:
  ingress:
  - from:
    - podSelector: {}
  podSelector: {}
  policyTypes:
  - Ingress
```

The second policy applied is to the `kube-system` namespace and allows for DNS traffic. See below.

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  managedFields:
  - apiVersion: networking.k8s.io/v1
    fieldsV1:
      f:spec:
        f:ingress: {}
        f:podSelector:
          f:matchLabels:
        f:policyTypes: {}
  name: default-network-dns-policy
  namespace: kube-system
spec:
  ingress:
  - ports:
    - port: 53
      protocol: TCP
    - port: 53
      protocol: UDP
  podSelector:
    matchLabels:
  policyTypes:
  - Ingress
```

RKE2 applies the `default-network-policy` policy and `np.rke2.io` annotation to all built-in namespaces. The `kube-system` namespace additionally gets the `default-network-dns-policy` policy and `np.rke2.io/dns` annotation applied to it.

To view the network policies currently deployed on your system, run the below command:

```bash
kubectl get networkpolicies -A
```
