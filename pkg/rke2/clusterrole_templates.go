package rke2

const (
	kubeletAPIServerRoleBindingName = "kube-apiserver-kubelet-admin"
	tunnelControllerRoleName        = "system:k3s-controller"
)

const kubeletAPIServerRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:kubelet-api-admin
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: kube-apiserver
`

const tunnelControllerRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %s
rules:
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - namespaces
  verbs:
  - list
  - watch
- apiGroups:
  - "networking.k8s.io"
  resources:
  - networkpolicies
  verbs:
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - endpoints
  - pods
  verbs:
  - list
  - get
  - watch
`

const tunnelControllerRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:k3s-controller
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %s
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: %s
`
