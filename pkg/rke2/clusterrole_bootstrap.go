package rke2

import (
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rbacv1helpers "k8s.io/kubernetes/pkg/apis/rbac/v1"
)

const (
	kubeletAPIServerRoleBindingName = "kube-apiserver-kubelet-admin"
	kubeProxyName                   = "system:kube-proxy"
	tunnelControllerName            = "system:rke2-controller"
	cloudControllerManagerName      = "rke2-cloud-controller-manager"

	appsGroup         = "apps"
	coordinationGroup = "coordination.k8s.io"
	helmGroup         = "helm.cattle.io"
	legacyGroup       = ""
	networkingGroup   = "networking.k8s.io"
	discoveryGroup    = "discovery.k8s.io"
)

var (
	label      = map[string]string{"rke2.io/bootstrapping": "rbac-defaults"}
	annotation = map[string]string{rbacv1.AutoUpdateAnnotationKey: "true"}
)

// clusterRoles returns a list of clusterrolebindings to bootstrap
func clusterRoles() []rbacv1.ClusterRole {
	roles := []rbacv1.ClusterRole{
		{
			ObjectMeta: metav1.ObjectMeta{Name: kubeProxyName},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get", "list").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: tunnelControllerName},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
				rbacv1helpers.NewRule("list", "watch").Groups(legacyGroup).Resources("namespaces").RuleOrDie(),
				rbacv1helpers.NewRule("list", "watch").Groups(networkingGroup).Resources("networkpolicies").RuleOrDie(),
				rbacv1helpers.NewRule("list", "watch", "get").Groups(legacyGroup).Resources("endpoints", "pods").RuleOrDie(),
				rbacv1helpers.NewRule("create").Groups(legacyGroup).Resources("serviceaccounts/token").RuleOrDie(),
				rbacv1helpers.NewRule("list", "get").Groups(helmGroup).Resources("helmcharts", "helmchartconfigs").RuleOrDie(),
			},
		},
		{
			// this should be kept in sync with the ClusterRole in k3s:
			// https://github.com/k3s-io/k3s/blob/master/manifests/ccm.yaml
			ObjectMeta: metav1.ObjectMeta{Name: cloudControllerManagerName},
			Rules: []rbacv1.PolicyRule{
				rbacv1helpers.NewRule("get", "create", "update").Groups(coordinationGroup).Resources("leases").RuleOrDie(),
				rbacv1helpers.NewRule("create", "patch", "update").Groups(legacyGroup).Resources("events").RuleOrDie(),
				rbacv1helpers.NewRule("*").Groups(legacyGroup).Resources("nodes").RuleOrDie(),
				rbacv1helpers.NewRule("patch").Groups(legacyGroup).Resources("nodes/status", "services/status").RuleOrDie(),
				rbacv1helpers.NewRule("get", "list", "watch", "patch", "update").Groups(legacyGroup).Resources("services", "pods").RuleOrDie(),
				rbacv1helpers.NewRule("create", "get").Groups(legacyGroup).Resources("serviceaccounts").RuleOrDie(),
				rbacv1helpers.NewRule("create", "get").Groups("").Resources("namespaces").RuleOrDie(),
				rbacv1helpers.NewRule("*").Groups(appsGroup).Resources("daemonsets").RuleOrDie(),
				rbacv1helpers.NewRule("get", "list", "watch").Groups(discoveryGroup).Resources("endpointslices").RuleOrDie(),
			},
		},
	}
	addClusterRoleLabel(roles)
	return roles
}

// clusterRoleBindings returns a list of clusterrolebindings to bootstrap
func clusterRoleBindings() []rbacv1.ClusterRoleBinding {
	rolebindings := []rbacv1.ClusterRoleBinding{
		// https://github.com/kubernetes/kubernetes/issues/65939#issuecomment-403218465
		ClusterRoleBindingName(rbacv1helpers.NewClusterBinding("system:kubelet-api-admin").Users("kube-apiserver"), kubeletAPIServerRoleBindingName).BindingOrDie(),
		ClusterRoleBindingNamespacedUsers(ClusterRoleBindingName(rbacv1helpers.NewClusterBinding("system:auth-delegator"), cloudControllerManagerName+"-auth-delegator"), metav1.NamespaceSystem, cloudControllerManagerName).BindingOrDie(),
		ClusterRoleBindingNamespacedUsers(rbacv1helpers.NewClusterBinding(cloudControllerManagerName), metav1.NamespaceSystem, cloudControllerManagerName).BindingOrDie(),
		rbacv1helpers.NewClusterBinding(kubeProxyName).Users(kubeProxyName).BindingOrDie(),
		rbacv1helpers.NewClusterBinding(tunnelControllerName).Users(tunnelControllerName).BindingOrDie(),
	}
	addClusterRoleBindingLabel(rolebindings)
	return rolebindings
}

// roles returns a map of namespace to roles to bootstrap into that namespace
func roles() map[string][]rbacv1.Role {
	return nil
}

// roleBindings returns a map of namespace to rolebindings to bootstrap into that namespace
func roleBindings() map[string][]rbacv1.RoleBinding {
	ccmAuthReader := RoleBindingNamespacedUsers(RoleBindingName(rbacv1helpers.NewRoleBinding("extension-apiserver-authentication-reader", metav1.NamespaceSystem), cloudControllerManagerName+"-authentication-reader"), metav1.NamespaceSystem, cloudControllerManagerName).BindingOrDie()
	addDefaultMetadata(&ccmAuthReader)
	return map[string][]rbacv1.RoleBinding{
		metav1.NamespaceSystem: {
			ccmAuthReader,
		},
	}
}

// RoleBindingNamespacedUsers adds namespaced users to a RoleBindingBuilder's Subjects list.
// For some reason the core helpers don't have any methods for adding namespaced users, only namespaced service accounts.
func RoleBindingNamespacedUsers(r *rbacv1helpers.RoleBindingBuilder, namespace string, users ...string) *rbacv1helpers.RoleBindingBuilder {
	for _, user := range users {
		r.RoleBinding.Subjects = append(r.RoleBinding.Subjects, rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Namespace: namespace, Name: user})
	}
	return r
}

// RoleBindingName sets the name on a RoleBindingBuilder's policy.
// The ClusterRoleBindingBuilder sets the ClusterRoleBinding name to the same as ClusterRole it's binding to without
// providing any way to override it.
func RoleBindingName(r *rbacv1helpers.RoleBindingBuilder, name string) *rbacv1helpers.RoleBindingBuilder {
	r.RoleBinding.ObjectMeta.Name = name
	return r
}

// ClusterRoleBindingNamespacedUsers adds namespaced users to a ClusterRoleBindingBuilder's Subjects list.
// For some reason the core helpers don't have any methods for adding namespaced users, only namespaced service accounts.
func ClusterRoleBindingNamespacedUsers(r *rbacv1helpers.ClusterRoleBindingBuilder, namespace string, users ...string) *rbacv1helpers.ClusterRoleBindingBuilder {
	for _, user := range users {
		r.ClusterRoleBinding.Subjects = append(r.ClusterRoleBinding.Subjects, rbacv1.Subject{APIGroup: rbacv1.GroupName, Kind: rbacv1.UserKind, Namespace: namespace, Name: user})
	}
	return r
}

// ClusterRoleBindingName sets the name on a ClusterRoleBindingBuilder's policy.
// The ClusterRoleBindingBuilder sets the ClusterRoleBinding name to the same as ClusterRole it's binding to without
// providing any way to override it.
func ClusterRoleBindingName(r *rbacv1helpers.ClusterRoleBindingBuilder, name string) *rbacv1helpers.ClusterRoleBindingBuilder {
	r.ClusterRoleBinding.ObjectMeta.Name = name
	return r
}

// cribbed from k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go
func addDefaultMetadata(obj runtime.Object) {
	metadata, err := meta.Accessor(obj)
	if err != nil {
		// if this happens, then some static code is broken
		panic(err)
	}

	labels := metadata.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	for k, v := range label {
		labels[k] = v
	}
	metadata.SetLabels(labels)

	annotations := metadata.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	for k, v := range annotation {
		annotations[k] = v
	}
	metadata.SetAnnotations(annotations)
}

// cribbed from k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go
func addClusterRoleLabel(roles []rbacv1.ClusterRole) {
	for i := range roles {
		addDefaultMetadata(&roles[i])
	}
}

// cribbed from k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go
func addClusterRoleBindingLabel(rolebindings []rbacv1.ClusterRoleBinding) {
	for i := range rolebindings {
		addDefaultMetadata(&rolebindings[i])
	}
}

// cribbed from k8s.io/kubernetes/plugin/pkg/auth/authorizer/rbac/bootstrappolicy/policy.go
func addRoleLabel(roles []rbacv1.Role) {
	for i := range roles {
		addDefaultMetadata(&roles[i])
	}
}
