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

// globalRestrictedPSP
const globalRestrictedPSP = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: %s
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: 'docker/default,runtime/default'
    apparmor.security.beta.kubernetes.io/allowedProfileNames: 'runtime/default'
    seccomp.security.alpha.kubernetes.io/defaultProfileName: 'runtime/default'
    apparmor.security.beta.kubernetes.io/defaultProfileName: 'runtime/default'
spec:
  privileged: false
  allowPrivilegeEscalation: false
  requiredDropCapabilities:
    - ALL
  volumes:
    - 'configMap'
    - 'emptyDir'
    - 'projected'
    - 'secret'
    - 'downwardAPI'
    - 'persistentVolumeClaim'
  hostNetwork: false
  hostIPC: false
  hostPID: false
  runAsUser:
    rule: 'MustRunAsNonRoot'
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
