package psp

// globalUnrestrictedPSPTemplate
const globalUnrestrictedPSPTemplate = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: %s
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
`

// globalUnrestrictedRoleBindingTemplate
const globalUnrestrictedRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %s
subjects:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:authenticated
`
