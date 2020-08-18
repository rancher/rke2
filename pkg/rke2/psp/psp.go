package psp

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rancher/spur/cli"
	"github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	"k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	yamlutil "k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/transport"
)

const (
	defaultRetries          = 5
	defaultWaitSeconds      = 5
	defaultTimeout          = 30
	k8sWrapTransportTimeout = 30
)

const (
	cisAnnotationValue = "resolved"

	namespaceAnnotationBase               = "psp.rke2.io/"
	namespaceAnnotationGlobalRestricted   = namespaceAnnotationBase + "global-restricted"
	namespaceAnnotationGlobalUnrestricted = namespaceAnnotationBase + "global-unrestricted"
	namespaceAnnotationSystemUnrestricted = namespaceAnnotationBase + "system-unrestricted"
)

// setGlobalUnrestricted sets the global unrestricted podSecurityPolicy with
// the associated role and rolebinding.
func setGlobalUnrestricted(ctx context.Context, cs *kubernetes.Clientset, ns *v1.Namespace) error {
	if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, globalUnrestrictedPSPName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(globalUnrestrictedPSPTemplate, globalUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			}
		}

		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, globalUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(roleTemplate, globalUnrestrictedRoleName, globalUnrestrictedPSPName)
				if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			}
		}

		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, globalUnrestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(globalUnrestrictedRoleBindingTemplate, globalUnrestrictedRoleBindingName, globalUnrestrictedRoleName)
				if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			}
		}

		ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalUnrestricted: cisAnnotationValue})
	}
	return nil
}

// setSystemUnrestricted sets the system unrestricted podSecurityPolicy as
// the associated role and rolebinding.
func setSystemUnrestricted(ctx context.Context, cs *kubernetes.Clientset, ns *v1.Namespace) error {
	if _, ok := ns.Annotations[namespaceAnnotationSystemUnrestricted]; !ok {
		if _, err := cs.RbacV1().ClusterRoles().Get(context.TODO(), systemUnrestrictedPSPName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(systemUnrestrictedPSPTemplate, systemUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			}
		}
		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, systemUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(roleTemplate, systemUnrestrictedRoleName, systemUnrestrictedPSPName)
				if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, systemUnrestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(systemUnrestrictedNodesRoleBindingTemplate, systemUnrestrictedRoleBindingName, systemUnrestrictedRoleName)
				if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, systemUnrestrictedSvcAcctRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				tmpl := fmt.Sprintf(systemUnrestrictedServiceAcctRoleBindingTemplate, systemUnrestrictedSvcAcctRoleBindingName, systemUnrestrictedRoleName)
				if err := deployRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
					return err
				}
			}
		}
		ns.SetAnnotations(map[string]string{namespaceAnnotationSystemUnrestricted: cisAnnotationValue})
	}
	return nil
}

// kubeConfigExists checks if the kubeconfig file
// has been written.
func kubeConfigExists(kc string) bool {
	info, err := os.Stat(kc)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}

// SetPSPs sets the default PSP's based on the mode that RKE2 is running in. There is either CIS or non
// CIS mode. For CIS mode, the globalRestricted and systemUnrestricted polices and associated roles and
// bindings are created. For non CIS mode, globalRestricted, system-unrestricted, and global-unrestricted
// policies and associated roles and bindings are loaded, if applicable.
//
// Each PSP has an annotation associated with it that we check to see exists before performing any
// write operations. The annotation is created upon successful setting of PSPs.
//
// Load logic
//
// CIS:
// - If the globalRestricted annotation does not exist, create PSP, role, binding.
// - If the systemUnrestricted annotation does not exist, create the PSP, role, binding.
// - If the globalUnrestricted annotation does not exist, check if PSP exists, and if so,
//   delete it, the role, and the bindings.
//
// non-CIS:
// - If the globalUnrestricted annotation does not exist, then create PSP, role, binding.
// - If the systemUnrestricted annotation does not exist, then create PSP, role, binding.
// - If the globalRestricted annotation does not exist, then check if the PSP exists and
//   if it doesn't, create it. Check if the associated role and bindings exist and
//   if they do, delete them.
func SetPSPs(sCtx context.Context, ctx *cli.Context, k8sWrapTransport transport.WrapperFunc) error {
	const (
		kubeConfigPath = "/etc/rancher/rke2/rke2.yaml"
		waitDelay      = time.Second * 1
	)

	// wait until the kubeconfig file is written
	for !kubeConfigExists(kubeConfigPath) {
		time.Sleep(waitDelay)
	}

	cs, err := newClient(kubeConfigPath, k8sWrapTransport)
	if err != nil {
		return err
	}
	var complete bool
	for complete {
		// wait until kube-apiserver is running
		if _, err := cs.Discovery().ServerVersion(); err != nil {
			time.Sleep(waitDelay)
			continue
		}

		// wait until the kube-system namespace is created
		ns, err := cs.CoreV1().Namespaces().Get(sCtx, metav1.NamespaceSystem, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				time.Sleep(waitDelay)
				continue
			}
			return err
		}

		if ctx.String("profile") == "" { // non-CIS mode
			if err := setGlobalUnrestricted(sCtx, cs, ns); err != nil {
				return err
			}

			if err := setSystemUnrestricted(sCtx, cs, ns); err != nil {
				return err
			}

			if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
				// check if the policy exists
				if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(sCtx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
						if err := deployPodSecurityPolicyFromYaml(sCtx, cs, tmpl); err != nil {
							return err
						}
					}
				}

				// check if role exists and delete it
				role, err := cs.RbacV1().ClusterRoles().Get(sCtx, globalRestrictedRoleName, metav1.GetOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
				if err := cs.RbacV1().ClusterRoles().Delete(sCtx, role.Name, metav1.DeleteOptions{}); err != nil {
					return err
				}

				// check if role binding exists and delete it
				roleBinding, err := cs.RbacV1().ClusterRoleBindings().Get(sCtx, globalRestrictedRoleBindingName, metav1.GetOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					logrus.Info(err)

				}
				if err := cs.RbacV1().ClusterRoleBindings().Delete(sCtx, roleBinding.Name, metav1.DeleteOptions{}); err != nil {
					return err
				}

				ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalRestricted: cisAnnotationValue})
			}
			complete = true
		} else { // CIS mode
			if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
				if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(sCtx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
					tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
					if err := deployPodSecurityPolicyFromYaml(sCtx, cs, tmpl); err != nil {
						return err
					}
				}
				if _, err := cs.RbacV1().ClusterRoles().Get(sCtx, globalRestrictedRoleName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(roleTemplate, globalRestrictedRoleName, globalRestrictedPSPName)
						if err := deployClusterRoleFromYaml(sCtx, cs, tmpl); err != nil {
							return err
						}
					}
				}
				if _, err := cs.RbacV1().ClusterRoleBindings().Get(sCtx, globalRestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(globalRestrictedRoleBindingTemplate, globalRestrictedRoleBindingName, globalRestrictedRoleName)
						if err := deployClusterRoleBindingFromYaml(sCtx, cs, tmpl); err != nil {
							return err
						}
					}
				}
				ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalRestricted: cisAnnotationValue})
			}

			if err := setSystemUnrestricted(sCtx, cs, ns); err != nil {
				return err
			}

			if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
				if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(sCtx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
						if err := deployPodSecurityPolicyFromYaml(sCtx, cs, tmpl); err != nil {
							return err
						}
					}
				}

				// check if role exists and if so, delete it
				role, err := cs.RbacV1().ClusterRoles().Get(sCtx, globalRestrictedRoleName, metav1.GetOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
				if err := cs.RbacV1().ClusterRoles().Delete(sCtx, role.Name, metav1.DeleteOptions{}); err != nil {
					return err
				}

				// check if role binding exists and if so, delete it
				roleBinding, err := cs.RbacV1().ClusterRoleBindings().Get(sCtx, globalUnrestrictedRoleName, metav1.GetOptions{})
				if err != nil && !apierrors.IsNotFound(err) {
					return err
				}
				if err := cs.RbacV1().ClusterRoleBindings().Delete(sCtx, roleBinding.Name, metav1.DeleteOptions{}); err != nil {
					return err
				}

				ns.SetAnnotations(map[string]string{namespaceAnnotationGlobalUnrestricted: cisAnnotationValue})
			}
			complete = true
		}

		// apply node cluster role binding regardless of whether we're in CIS mode or not
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(sCtx, nodeClusterRoleBindingTemplate, metav1.GetOptions{}); err != nil {
			if apierrors.IsAlreadyExists(err) {
				logrus.Info(err)
			} else {
				tmpl := fmt.Sprintf(nodeClusterRoleBindingTemplate, globalUnrestrictedPSPName)
				if err := deployClusterRoleBindingFromYaml(sCtx, cs, tmpl); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

type deployFn func(context.Context, *kubernetes.Clientset, interface{}) error

// newClient create a new Kubernetes client from configuration.
func newClient(kubeConfigPath string, k8sWrapTransport transport.WrapperFunc) (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeConfigPath)
	if err != nil {
		return nil, err
	}
	if k8sWrapTransport != nil {
		config.WrapTransport = k8sWrapTransport
	}
	config.Timeout = time.Second * defaultTimeout
	return kubernetes.NewForConfig(config)
}

func decodeYamlResource(data interface{}, yaml string) error {
	decoder := yamlutil.NewYAMLToJSONDecoder(bytes.NewReader([]byte(yaml)))
	return decoder.Decode(data)
}

func retryTo(runFunc deployFn, ctx context.Context, cs *kubernetes.Clientset, resource interface{}, retries, wait int) error {
	var err error
	if retries <= 0 {
		retries = defaultRetries
	}
	if wait <= 0 {
		wait = defaultWaitSeconds
	}
	for i := 0; i < retries; i++ {
		if err = runFunc(ctx, cs, resource); err != nil {
			time.Sleep(time.Second * time.Duration(wait))
			continue
		}
		return nil
	}
	return err
}

func deployPodSecurityPolicyFromYaml(ctx context.Context, cs *kubernetes.Clientset, pspYaml string) error {
	var psp v1beta1.PodSecurityPolicy
	if err := decodeYamlResource(&psp, pspYaml); err != nil {
		return err
	}
	return retryTo(deployPodSecurityPolicy, ctx, cs, psp, defaultRetries, defaultWaitSeconds)
}

func deployPodSecurityPolicy(ctx context.Context, cs *kubernetes.Clientset, p interface{}) error {
	psp, ok := p.(v1beta1.PodSecurityPolicy)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: PodSecurityPolicy", p)
	}
	if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Create(ctx, &psp, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Update(context.TODO(), &psp, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func deployClusterRoleBindingFromYaml(ctx context.Context, cs *kubernetes.Clientset, clusterRoleBindingYaml string) error {
	var clusterRoleBinding rbacv1.ClusterRoleBinding
	if err := decodeYamlResource(&clusterRoleBinding, clusterRoleBindingYaml); err != nil {
		return err
	}
	return retryTo(deployClusterRoleBinding, ctx, cs, clusterRoleBinding, defaultRetries, defaultWaitSeconds)
}

func deployClusterRoleBinding(ctx context.Context, cs *kubernetes.Clientset, crb interface{}) error {
	clusterRoleBinding, ok := crb.(rbacv1.ClusterRoleBinding)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: ClusterRoleBinding", crb)
	}
	if _, err := cs.RbacV1().ClusterRoleBindings().Create(ctx, &clusterRoleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Update(ctx, &clusterRoleBinding, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func deployClusterRoleFromYaml(ctx context.Context, cs *kubernetes.Clientset, clusterRoleYaml string) error {
	var clusterRole rbacv1.ClusterRole
	if err := decodeYamlResource(&clusterRole, clusterRoleYaml); err != nil {
		return err
	}
	return retryTo(deployClusterRole, ctx, cs, clusterRole, defaultRetries, defaultWaitSeconds)
}

func deployClusterRole(ctx context.Context, cs *kubernetes.Clientset, cr interface{}) error {
	clusterRole, ok := cr.(rbacv1.ClusterRole)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: ClusterRole", cr)
	}
	if _, err := cs.RbacV1().ClusterRoles().Create(ctx, &clusterRole, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().ClusterRoles().Update(ctx, &clusterRole, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func deployRoleBindingFromYaml(ctx context.Context, cs *kubernetes.Clientset, roleBindingYaml string) error {
	var roleBinding rbacv1.RoleBinding
	if err := decodeYamlResource(&roleBinding, roleBindingYaml); err != nil {
		return err
	}
	return retryTo(deployRoleBinding, ctx, cs, roleBinding, defaultRetries, defaultWaitSeconds)
}

func deployRoleBinding(ctx context.Context, cs *kubernetes.Clientset, rb interface{}) error {
	roleBinding, ok := rb.(rbacv1.RoleBinding)
	if !ok {
		return fmt.Errorf("invalid type provided: %T, expected: RoleBinding", rb)
	}
	if _, err := cs.RbacV1().RoleBindings(roleBinding.Namespace).Create(ctx, &roleBinding, metav1.CreateOptions{}); err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return err
		}
		if _, err := cs.RbacV1().RoleBindings(roleBinding.Namespace).Update(ctx, &roleBinding, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}
