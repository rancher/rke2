package rke2

const (
	globalUnrestrictedPSPName         = "global-unrestricted-psp"
	globalUnrestrictedRoleName        = "global-unrestricted-psp-clusterrole"
	globalUnrestrictedRoleBindingName = "global-unrestricted-psp-rolebinding"

	globalRestrictedPSPName         = "global-restricted-psp"
	globalRestrictedRoleName        = "global-restricted-psp-clusterrole"
	globalRestrictedRoleBindingName = "global-restricted-psp-rolebinding"

	systemUnrestrictedPSPName                = "system-unrestricted-psp"
	systemUnrestrictedRoleName               = "system-unrestricted-psp-role"
	systemUnrestrictedRoleBindingName        = "system-unrestricted-node-psp-rolebinding"
	systemUnrestrictedSvcAcctRoleBindingName = "system-unrestricted-svc-acct-psp-rolebinding"

	nodeClusterRoleBindingName = "system-node-default-psp-rolebinding"
)

const roleTemplate = `kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: %s
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - %s
`

const globalRestrictedPSPTemplate = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: %s
  annotations:
    psp.rke2.io/global-restricted: resolved
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
`

const globalRestrictedRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
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

const globalUnrestrictedPSPTemplate = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: %s
  annotations:
    psp.rke2.io/global-unrestricted: resolved
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

const systemUnrestrictedPSPTemplate = `apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: %s
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

const systemUnrestrictedServiceAcctRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: system-unrestricted-svc-acct-psp-rolebinding
  namespace: kube-system
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system-unrestricted-psp-role
subjects:
  - kind: Group
    apiGroup: rbac.authorization.k8s.io
    name: system:serviceaccounts
`

const nodeClusterRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system-node-default-psp-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %s
subjects:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:nodes
`
