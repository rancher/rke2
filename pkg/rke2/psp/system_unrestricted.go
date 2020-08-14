package psp

// dupe of globalUnrestructed but managed differently
// - regardless of the mdoe we're in, we want a special
//   PSP for the components we need to manage.
//   Scenario: NOT in CIS
//     -
// systemUnrestrictedPSPTemplate
const systemUnrestrictedPSPTemplate = `
apiVersion: policy/v1beta1
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

// systemUnrestrictedNodesRoleBindingTemplate
const systemUnrestrictedNodesRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
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
  name: system:nodes
`

// systemUnrestrictedServiceAcctRoleBindingTemplate
const systemUnrestrictedServiceAcctRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %s
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %s
subjects:
  - kind: Group
    apiGroup: rbac.authorization.k8s.io
    name: system:serviceaccounts
`
