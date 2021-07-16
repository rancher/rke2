package rke2

const (
	kubeletAPIServerRoleBindingName = "kube-apiserver-kubelet-admin"
	kubeProxyRoleName               = "system:kube-proxy"
	tunnelControllerRoleName        = "system:rke2-controller"
	cloudControllerManagerRoleName  = "rke2-cloud-controller-manager"
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

const kubeProxyServerRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %[1]s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %[1]s
subjects:
- apiGroup: rbac.authorization.k8s.io
  kind: User
  name: %[1]s
`

const kubeProxyRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
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
    - list
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
- apiGroups:
  - ""
  resources:
  - secrets
  verbs:
  - list
  - get
- apiGroups:
  - "helm.cattle.io"
  resources:
  - helmcharts
  - helmchartconfigs
  verbs:
  - list
  - get
`

const tunnelControllerRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %[1]s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %[1]s
subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: %[1]s
`

const cloudControllerManagerRoleBindingTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: %[1]s
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: %[1]s
subjects:
  - kind: User
    name: %[1]s
    namespace: kube-system
`

const cloudControllerManagerRoleTemplate = `apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: %s
rules:
  - apiGroups:
      - coordination.k8s.io
    resources:
      - leases
    verbs:
      - get
      - create
      - update
  - apiGroups:
      - ""
    resources:
      - events
    verbs:
      - create
      - patch
      - update
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - '*'
  - apiGroups:
      - ""
    resources:
      - nodes/status
    verbs:
      - patch
  - apiGroups:
      - ""
    resources:
      - services
    verbs:
      - list
      - patch
      - update
      - watch
  - apiGroups:
      - ""
    resources:
      - serviceaccounts
    verbs:
      - create
  - apiGroups:
      - ""
    resources:
      - persistentvolumes
    verbs:
      - get
      - list
      - update
      - watch
  - apiGroups:
      - ""
    resources:
      - endpoints
    verbs:
      - create
      - get
      - list
      - watch
      - update
`
