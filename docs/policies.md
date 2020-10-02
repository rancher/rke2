---
title: General Operation and CIS Security Policies
weight: 204
---

#### Pod Security Policies

RKE2 can be run with or without the `--profile=cis-1.5` argument and behaves differently when done so in regards to the PodSecurityPolicies it applies at start-up.

At start, RKE2 checks to see if it is running in CIS mode and if so checks for the existence of 3 annotations applied to the "kube-system" namespace in this order; `globalRestricted`, and `systemUnrestricted`, and `globalUnrestricted`. If the annotation exists, RKE2 continues. If any of the 3 annotations don't exist, we check to see if the policy exists and if so, we delete it and the associated policy is created as well as the annotation. However the "globalUnrestricted" policy is just deleted. This is to account for moving to and from CIS mode and non CIS mode. At the time of creating the policies, cluster roles and cluster role bindings are checked for existence and created if necessary. They're then associated with the relevant polciy. See below.

```yaml
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: global-restricted-psp
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: 'docker/default,runtime/default'
    apparmor.security.beta.kubernetes.io/allowedProfileNames: 'runtime/default'
    seccomp.security.alpha.kubernetes.io/defaultProfileName: 'runtime/default'
    apparmor.security.beta.kubernetes.io/defaultProfileName: 'runtime/default'
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
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: '*'
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
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: '*'
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

When RKE2 is run with the `--profile=cis-1.5` argument, it will apply 2 network policies to the "kube-system", "kube-public", and "default" namespaces and applies associated annotations. On start, the annotations for each namespace are checked for existence and if they exist, we take no action. If the annoation doesn't exist, we check to see if the policy exists and if it does, we delete it, and create the new policy.

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
    manager: rke2
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

The second policy applied is to the "kube-system" namespace and allows for DNS traffic. See below.

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
    manager: rke2
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

RKE2 applies 2 network policy for all namespaces and updates the namespaces with annotations `default-network-policy` and `default-network-dns-policy`. 

To view the network policies currently deployed on your system, run the below command:

```bash
kubectl get networkpolicies -A
```
