package rke2

const (
	globalUnrestrictedPSPName         = "global-unrestricted-psp"
	globalUnrestrictedRoleName        = "global-unrestricted-psp-role"
	globalUnrestrictedRoleBindingName = "global-unrestricted-psp-rolebinding"

	globalRestrictedPSPName         = "global-restricted-psp"
	globalRestrictedRoleName        = "global-restricted-psp-role"
	globalRestrictedRoleBindingName = "global-restricted-psp-rolebinding"
)

// globalUnrestrictedPSP
const globalUnrestrictedPSP = `apiVersion: policy/v1beta1
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

// globalUnrestrictedRole
const globalUnrestrictedRole = `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: %s
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - default-psp
`

// globalUnrestrictedRoleBinding
const globalUnrestrictedRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %s
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: default-psp-role
subjects:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:authenticated
`

// globalUnrestrictedPSP
const globalRestrictedPSP = `apiVersion: policy/v1beta1
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

// globalRestrictedRole
const globalRestrictedRole = `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: %s
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - default-psp
`

// globalRestrictedRoleBinding
const globalRestrictedRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: %s
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: default-psp-role
subjects:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:authenticated
`

// nodeClusterRoleBinding
const nodeClusterRoleBinding = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system-node-default-psp-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: default-psp-role
subjects:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:nodes
`
