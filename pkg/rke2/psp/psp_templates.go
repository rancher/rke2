package psp

const (
	globalUnrestrictedPSPName         = "global-unrestricted-psp"
	globalUnrestrictedRoleName        = "global-unrestricted-psp-role"
	globalUnrestrictedRoleBindingName = "global-unrestricted-psp-rolebinding"

	globalRestrictedPSPName         = "global-restricted-psp"
	globalRestrictedRoleName        = "global-restricted-psp-role"
	globalRestrictedRoleBindingName = "global-restricted-psp-rolebinding"

	systemUnrestrictedPSPName         = "system-restricted-psp"
	systemUnrestrictedRoleName        = "system-restricted-psp-role"
	systemUnrestrictedRoleBindingName = "system-restricted-psp-rolebinding"
)

// roleTemplate
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

// nodeClusterRoleBindingTemplate
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
