---
title: Default Policy Configuration
---
This document describes how RKE2 configures PodSecurityPolicies and NetworkPolicies in order to be secure-by-default while also providing operators with maximum configuration flexibility.

#### Pod Security Policies

RKE2 can be ran with or without the `profile: cis-1.5` configuration parameter. This will cause it to apply different PodSecurityPolicies (PSPs) at start-up.

* If running with the `cis-1.5` profile, RKE2 will apply a restrictive policy called `global-restricted-psp` to all namespaces except `kube-system`. The `kube-system` namespace needs a less restrictive policy named `system-unrestricted-psp` in order to launch critical components.
* If running without the `cis-1.5` profile, RKE2 will apply a completely unrestricted policy called `global-unrestricted-psp`, which is the equivalent of running without the PSP admission controller enabled.

RKE2 will put these policies in place upon initial startup, but will not modify them after that, unless explicitly triggered by the cluster operator as described below. This is to allow the operator to fully control the PSPs without RKE2's defaults adding interference.

The creation and application of the PSPs are controlled by the presence or absence of certain annotations on the `kube-system` namespace. These map directly to the PSPs which can be created and are:

 * `psp.rke2.io/global-restricted`
 * `psp.rke2.io/system-unrestricted`
 * `psp.rke2.io/global-unrestricted`

The following logic is performed at startup for the policies and their annotations:

* If the annotation exists, RKE2 continues without further action.
* If the annotation doesn't exist, RKE2 checks to see if the associated policy exists and if so, deletes and recreates it, along with adding the annotation to the namespace.
* In the case of the `global-unrestricted-psp`, the policy is not recreated. This is to account for moving between CIS and non-CIS modes without making the cluster less secure.
* At the time of creating a policy, cluster roles and cluster role bindings are also created to ensure the appropriate policies are put into use by default.

So, after the initial start-up, operators can modify or delete RKE2's policies and RKE2 will respect those changes. Additionally, to "reset" a policy, an operator just needs to delete the associated annotation from the `kube-system` namespace and restart RKE2.

The policies are outlined below.

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: global-restricted-psp
spec:
  privileged: false                # CIS - 5.2.1
  allowPrivilegeEscalation: false  # CIS - 5.2.5
  requiredDropCapabilities:        # CIS - 5.2.7/8/9
    - ALL
  volumes:
    - 'configMap'
    - 'emptyDir'
    - 'projected'
    - 'secret'
    - 'downwardAPI'
    - 'persistentVolumeClaim'
  hostNetwork: false               # CIS - 5.2.4
  hostIPC: false                   # CIS - 5.2.3
  hostPID: false                   # CIS - 5.2.2
  runAsUser:
    rule: 'MustRunAsNonRoot'       # CIS - 5.2.6
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'MustRunAs'
    ranges:
      - min: 1
        max: 65535
  fsGroup:
    rule: 'MustRunAs'
    ranges:
      - min: 1
        max: 65535
  readOnlyRootFilesystem: false
```

If RKE2 is started in non CIS mode, annotations are checked like above however the resulting application of pod security policies is a permissive one. See below.

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: global-unrestricted-psp
spec:
  privileged: true
  allowPrivilegeEscalation: true
  allowedCapabilities:
  - '*'
  volumes:
  - '*'
  hostNetwork: true
  hostPorts:
  - min: 0
    max: 65535
  hostIPC: true
  hostPID: true
  runAsUser:
    rule: 'RunAsAny'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'RunAsAny'
  fsGroup:
    rule: 'RunAsAny'
```

In both cases, the "system unrestricted policy" is applied. See below.

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: system-unrestricted-psp
spec:
  privileged: true
  allowPrivilegeEscalation: true
  allowedCapabilities:
  - '*'
  volumes:
  - '*'
  hostNetwork: true
  hostPorts:
  - min: 0
    max: 65535
  hostIPC: true
  hostPID: true
  runAsUser:
    rule: 'RunAsAny'
  seLinux:
    rule: 'RunAsAny'
  supplementalGroups:
    rule: 'RunAsAny'
  fsGroup:
    rule: 'RunAsAny'
```

To view the podSecurityPolicies currently deployed on your system, run the below command:

```bash
kubectl get psp -A
```

#### Network Policies

When RKE2 is run with the `profile: cis-1.5` parameter, it will apply 2 network policies to the `kube-system`, `kube-public`, and `default` namespaces and applies associated annotations. The same logic applies to these policies and annotations as the PSPs. On start, the annotations for each namespace are checked for existence and if they exist, RKE2 takes no action. If the annotation doesn't exist, RKE2 checks to see if the policy exists and if it does, recreates it.

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
