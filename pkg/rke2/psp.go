package rke2

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/rancher/k3s/pkg/cli/cmds"

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
	"k8s.io/client-go/util/retry"
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
func setGlobalUnrestricted(ctx context.Context, cs *kubernetes.Clientset, ns *v1.Namespace) (bool, error) {
	if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
		if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, globalUnrestrictedPSPName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting PSP: %s", globalUnrestrictedPSPName)
				tmpl := fmt.Sprintf(globalUnrestrictedPSPTemplate, globalUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
					return false, err
				}
			} else {
				return false, err
			}
		}
		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, globalUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster Role: %s", globalUnrestrictedRoleName)
				tmpl := fmt.Sprintf(roleTemplate, globalUnrestrictedRoleName, globalUnrestrictedPSPName)
				if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
					return false, err
				}
			} else {
				return false, err
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, globalUnrestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster RoleBinding: %s", globalUnrestrictedRoleBindingName)
				tmpl := fmt.Sprintf(globalUnrestrictedRoleBindingTemplate, globalUnrestrictedRoleBindingName, globalUnrestrictedRoleName)
				if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
					return false, err
				}
			} else {
				return false, err
			}
		}
		ns.Annotations[namespaceAnnotationGlobalUnrestricted] = cisAnnotationValue
		return true, nil
	}
	return false, nil
}

// setSystemUnrestricted sets the system unrestricted podSecurityPolicy as
// the associated role and rolebinding.
func setSystemUnrestricted(ctx context.Context, cs *kubernetes.Clientset, ns *v1.Namespace) (bool, error) {
	if _, ok := ns.Annotations[namespaceAnnotationSystemUnrestricted]; !ok {
		if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, systemUnrestrictedPSPName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting PSP: %s", systemUnrestrictedPSPName)
				tmpl := fmt.Sprintf(systemUnrestrictedPSPTemplate, systemUnrestrictedPSPName)
				if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
					return false, err
				}
			} else {
				return false, err
			}
		}
		if _, err := cs.RbacV1().ClusterRoles().Get(ctx, systemUnrestrictedRoleName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster Role: %s", systemUnrestrictedRoleName)
				tmpl := fmt.Sprintf(roleTemplate, systemUnrestrictedRoleName, systemUnrestrictedPSPName)
				if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
					return false, err
				}
			} else {
				return false, err
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, systemUnrestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster RoleBinding: %s", systemUnrestrictedRoleBindingName)
				tmpl := fmt.Sprintf(systemUnrestrictedNodesRoleBindingTemplate, systemUnrestrictedRoleBindingName, systemUnrestrictedRoleName)
				if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
					return false, err
				}
			} else {
				return false, err
			}
		}
		if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, systemUnrestrictedSvcAcctRoleBindingName, metav1.GetOptions{}); err != nil {
			if apierrors.IsNotFound(err) {
				logrus.Infof("Setting Cluster RoleBinding: %s", systemUnrestrictedSvcAcctRoleBindingName)
				if err := deployRoleBindingFromYaml(ctx, cs, systemUnrestrictedServiceAcctRoleBindingTemplate); err != nil {
					return false, err
				}
			} else {
				return false, err
			}
		}
		ns.Annotations[namespaceAnnotationSystemUnrestricted] = cisAnnotationValue
		return true, nil
	}
	return false, nil
}

// setPSPs sets the default PSP's based on the mode that RKE2 is running in. There is either CIS or non
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
func setPSPs(cisMode bool) cmds.StartupHook {
	return func(ctx context.Context, wg *sync.WaitGroup, args cmds.StartupHookArgs) error {
		go func() {
			defer wg.Done()
			<-args.APIServerReady
			logrus.Info("Applying Pod Security Policies")

			nsChanged := false
			cs, err := newClient(args.KubeConfigAdmin, nil)
			if err != nil {
				logrus.Fatalf("psp: new k8s client: %s", err.Error())
			}

			ns, err := cs.CoreV1().Namespaces().Get(ctx, metav1.NamespaceSystem, metav1.GetOptions{})
			if err != nil {
				logrus.Fatalf("psp: get namespace %s: %s", metav1.NamespaceSystem, err.Error())
			}
			if ns.Annotations == nil {
				ns.Annotations = make(map[string]string)
				nsChanged = true
			}

			if !cisMode { // non-CIS mode
				if changed, err := setGlobalUnrestricted(ctx, cs, ns); err != nil {
					logrus.Fatalf("psp: set globalUnrestricted: %s", err.Error())
				} else if changed {
					nsChanged = true
				}

				if changed, err := setSystemUnrestricted(ctx, cs, ns); err != nil {
					logrus.Fatalf("psp: set systemUnrestricted: %s", err.Error())
				} else if changed {
					nsChanged = true
				}

				if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
					// check if the policy exists
					if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
							if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
								logrus.Fatalf("psp: deploy psp: %s", err.Error())
							}
						} else {
							logrus.Fatalf("psp: get psp: %s", err.Error())
						}
					}

					// check if role exists and delete it
					_, err := cs.RbacV1().ClusterRoles().Get(ctx, globalRestrictedRoleName, metav1.GetOptions{})
					if err != nil {
						switch {
						case apierrors.IsAlreadyExists(err):
							logrus.Infof("Deleting clusterRole: %s", globalRestrictedRoleName)
							if err := cs.RbacV1().ClusterRoles().Delete(ctx, globalRestrictedRoleName, metav1.DeleteOptions{}); err != nil {
								logrus.Fatalf("psp: delete clusterrole: %s", err.Error())
							}
						case apierrors.IsNotFound(err):
							break
						default:
							logrus.Fatalf("psp: get clusterrole: %s", err.Error())
						}
					}

					// check if role binding exists and delete it
					_, err = cs.RbacV1().ClusterRoleBindings().Get(ctx, globalRestrictedRoleBindingName, metav1.GetOptions{})
					if err != nil && !apierrors.IsNotFound(err) {
						logrus.Fatalf("psp: get clusterrolebinding: %s", err.Error())
					}
					if err != nil {
						switch {
						case apierrors.IsAlreadyExists(err):
							logrus.Infof("Deleting clusterRole binding: %s", globalRestrictedRoleBindingName)
							if err := cs.RbacV1().ClusterRoleBindings().Delete(ctx, globalRestrictedRoleBindingName, metav1.DeleteOptions{}); err != nil {
								logrus.Fatalf("psp: delete clusterrolebinding: %s", err.Error())
							}
						case apierrors.IsNotFound(err):
							break
						default:
							logrus.Fatalf("psp: get clusterrolebinding: %s", err.Error())
						}
					}
					ns.Annotations[namespaceAnnotationGlobalRestricted] = cisAnnotationValue
					nsChanged = true
				}

				if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, nodeClusterRoleBindingName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(nodeClusterRoleBindingTemplate, globalUnrestrictedRoleName)
						if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
							logrus.Fatalf("psp: deploy clusterrolebinding: %s", err.Error())
						}
					} else {
						logrus.Fatalf("psp: get clusterrole binding: %s", err.Error())
					}
				}
			} else { // CIS mode
				if _, ok := ns.Annotations[namespaceAnnotationGlobalRestricted]; !ok {
					if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
						tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
						if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
							logrus.Fatalf("psp: deploy psp: %s", err.Error())
						}
					}
					if _, err := cs.RbacV1().ClusterRoles().Get(ctx, globalRestrictedRoleName, metav1.GetOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							tmpl := fmt.Sprintf(roleTemplate, globalRestrictedRoleName, globalRestrictedPSPName)
							if err := deployClusterRoleFromYaml(ctx, cs, tmpl); err != nil {
								logrus.Fatalf("psp: deploy clusterrole: %s", err.Error())
							}
						} else {
							logrus.Fatalf("psp: get clusterrole: %s", err.Error())
						}
					}
					if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, globalRestrictedRoleBindingName, metav1.GetOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							tmpl := fmt.Sprintf(globalRestrictedRoleBindingTemplate, globalRestrictedRoleBindingName, globalRestrictedRoleName)
							if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
								logrus.Fatalf("psp: deploy clusterrolebinding: %s", err.Error())
							}
						} else {
							logrus.Fatalf("psp: get clusterrolebinding: %s", err.Error())
						}
					}
					ns.Annotations[namespaceAnnotationGlobalRestricted] = cisAnnotationValue
					nsChanged = true
				}

				if changed, err := setSystemUnrestricted(ctx, cs, ns); err != nil {
					logrus.Fatalf("psp: set systemUnrestricted: %s", err.Error())
				} else if changed {
					nsChanged = true
				}

				if _, ok := ns.Annotations[namespaceAnnotationGlobalUnrestricted]; !ok {
					if _, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, globalRestrictedPSPName, metav1.GetOptions{}); err != nil {
						if apierrors.IsNotFound(err) {
							tmpl := fmt.Sprintf(globalRestrictedPSPTemplate, globalRestrictedPSPName)
							if err := deployPodSecurityPolicyFromYaml(ctx, cs, tmpl); err != nil {
								logrus.Fatalf("psp: deploy psp: %s", err.Error())
							}
						} else {
							logrus.Fatalf("psp: get psp: %s", err.Error())
						}
					}

					// check if role exists and if so, delete it
					_, err := cs.RbacV1().ClusterRoles().Get(ctx, globalUnrestrictedRoleName, metav1.GetOptions{})
					if err != nil {
						switch {
						case apierrors.IsAlreadyExists(err):
							logrus.Infof("Deleting clusterRole: %s", globalUnrestrictedRoleName)
							if err := cs.RbacV1().ClusterRoles().Delete(ctx, globalUnrestrictedRoleName, metav1.DeleteOptions{}); err != nil {
								logrus.Fatalf("psp: delete clusterrole: %s", err.Error())
							}
						case apierrors.IsNotFound(err):
							break
						default:
							logrus.Fatalf("psp: get clusterrole: %s", err.Error())
						}
					}

					// check if role binding exists and if so, delete it
					_, err = cs.RbacV1().ClusterRoleBindings().Get(ctx, globalUnrestrictedRoleBindingName, metav1.GetOptions{})
					if err != nil {
						switch {
						case apierrors.IsAlreadyExists(err):
							logrus.Infof("Deleting clusterRoleBinding: %s", globalUnrestrictedRoleBindingName)
							if err := cs.RbacV1().ClusterRoleBindings().Delete(ctx, globalUnrestrictedRoleBindingName, metav1.DeleteOptions{}); err != nil {
								logrus.Fatalf("psp: delete clusterrolebinding: %s", err.Error())
							}
						case apierrors.IsNotFound(err):
							break
						default:
							logrus.Fatalf("psp: get clusterrolebinding: %s", err.Error())
						}
					}
					ns.Annotations[namespaceAnnotationGlobalUnrestricted] = cisAnnotationValue
					nsChanged = true
				}

				if _, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, nodeClusterRoleBindingName, metav1.GetOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						tmpl := fmt.Sprintf(nodeClusterRoleBindingTemplate, globalRestrictedRoleName)
						if err := deployClusterRoleBindingFromYaml(ctx, cs, tmpl); err != nil {
							logrus.Fatalf("psp: deploy clusterrolebinding: %s", err.Error())
						}
					} else {
						logrus.Fatalf("psp: get clusterrolebinding: %s", err.Error())
					}
				}
			}

			if nsChanged {
				logrus.Infof("Updating annotations on %s namespace", ns.Name)
				if err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
					if err := updateNamespaceRef(ctx, cs, ns); err != nil {
						return err
					}
					_, err := cs.CoreV1().Namespaces().Update(ctx, ns, metav1.UpdateOptions{})
					return err
				}); err != nil {
					logrus.Fatalf("psp: update namespace: %s - %s", ns.Name, err.Error())
				}
			}

			logrus.Info("Pod Security Policies applied successfully")
		}()
		return nil
	}
}

// updateNamespaceRef retrieves the most recent revision of Namespace ns, copies over any annotations from
// the passed revision of the Namespace to the most recent revision, and updates the pointer to refer to the
// most recent revision. This get/change/update pattern is required to alter an object
// that may have changed since it was retrieved.
func updateNamespaceRef(ctx context.Context, cs *kubernetes.Clientset, ns *v1.Namespace) error {
	logrus.Info("updating namespace: " + ns.Name)
	newNS, err := cs.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if newNS.Annotations == nil {
		newNS.Annotations = make(map[string]string, len(ns.Annotations))
	}
	// copy annotations, since we may have changed them
	for k, v := range ns.Annotations {
		newNS.Annotations[k] = v
	}
	*ns = *newNS
	return nil
}

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

func deployPodSecurityPolicyFromYaml(ctx context.Context, cs kubernetes.Interface, pspYaml string) error {
	var psp v1beta1.PodSecurityPolicy
	if err := decodeYamlResource(&psp, pspYaml); err != nil {
		return err
	}

	// try to create the given PSP. If it already exists, we fall through to
	// attempting to update the existing PSP.
	if err := retry.OnError(retry.DefaultBackoff,
		func(err error) bool {
			return !apierrors.IsAlreadyExists(err)
		}, func() error {
			_, err := cs.PolicyV1beta1().PodSecurityPolicies().Create(ctx, &psp, metav1.CreateOptions{})
			return err
		},
	); err != nil && apierrors.IsAlreadyExists(err) {
		return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			retrievedPSP, err := cs.PolicyV1beta1().PodSecurityPolicies().Get(ctx, psp.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if retrievedPSP.Annotations == nil {
				retrievedPSP.Annotations = make(map[string]string, len(psp.Annotations))
			}
			for k, v := range psp.Annotations {
				retrievedPSP.Annotations[k] = v
			}
			retrievedPSP.Spec = psp.Spec
			_, err = cs.PolicyV1beta1().PodSecurityPolicies().Update(ctx, retrievedPSP, metav1.UpdateOptions{})
			return err
		})
	} else if err != nil {
		return err
	}
	return nil
}

func deployClusterRoleBindingFromYaml(ctx context.Context, cs kubernetes.Interface, clusterRoleBindingYaml string) error {
	var clusterRoleBinding rbacv1.ClusterRoleBinding
	if err := decodeYamlResource(&clusterRoleBinding, clusterRoleBindingYaml); err != nil {
		return err
	}

	// try to create the given cluster role binding. If it already exists, we
	// fall through to attempting to update the existing cluster role binding.
	if err := retry.OnError(retry.DefaultBackoff,
		func(err error) bool {
			return !apierrors.IsAlreadyExists(err)
		}, func() error {
			_, err := cs.RbacV1().ClusterRoleBindings().Create(ctx, &clusterRoleBinding, metav1.CreateOptions{})
			return err
		},
	); err != nil && apierrors.IsAlreadyExists(err) {
		return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			retrievedCRB, err := cs.RbacV1().ClusterRoleBindings().Get(ctx, clusterRoleBinding.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if retrievedCRB.Annotations == nil {
				retrievedCRB.Annotations = make(map[string]string, len(clusterRoleBinding.Annotations))
			}
			for k, v := range clusterRoleBinding.Annotations {
				retrievedCRB.Annotations[k] = v
			}
			retrievedCRB.Subjects = clusterRoleBinding.Subjects
			retrievedCRB.RoleRef = clusterRoleBinding.RoleRef
			_, err = cs.RbacV1().ClusterRoleBindings().Update(ctx, retrievedCRB, metav1.UpdateOptions{})
			return err
		})
	} else if err != nil {
		return err
	}
	return nil
}

func deployClusterRoleFromYaml(ctx context.Context, cs kubernetes.Interface, clusterRoleYaml string) error {
	var clusterRole rbacv1.ClusterRole
	if err := decodeYamlResource(&clusterRole, clusterRoleYaml); err != nil {
		return err
	}

	// try to create the given cluster role. If it already exists, we
	// fall through to attempting to update the existing cluster role.
	if err := retry.OnError(retry.DefaultBackoff,
		func(err error) bool {
			return !apierrors.IsAlreadyExists(err)
		}, func() error {
			_, err := cs.RbacV1().ClusterRoles().Create(ctx, &clusterRole, metav1.CreateOptions{})
			return err
		},
	); err != nil && apierrors.IsAlreadyExists(err) {
		return retry.RetryOnConflict(retry.DefaultRetry, func() error {
			retrievedCR, err := cs.RbacV1().ClusterRoles().Get(ctx, clusterRole.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if retrievedCR.Annotations == nil {
				retrievedCR.Annotations = make(map[string]string, len(clusterRole.Annotations))
			}
			for k, v := range clusterRole.Annotations {
				retrievedCR.Annotations[k] = v
			}
			retrievedCR.Rules = clusterRole.Rules
			_, err = cs.RbacV1().ClusterRoles().Update(ctx, retrievedCR, metav1.UpdateOptions{})
			return err
		})
	} else if err != nil {
		return err
	}
	return nil
}

func deployRoleBindingFromYaml(ctx context.Context, cs kubernetes.Interface, roleBindingYaml string) error {
	var roleBinding rbacv1.RoleBinding
	if err := decodeYamlResource(&roleBinding, roleBindingYaml); err != nil {
		return err
	}

	// try to create the given role binding. If it already exists, we fall through to
	// attempting to update the existing role binding.
	if err := retry.OnError(retry.DefaultBackoff,
		func(err error) bool {
			return !apierrors.IsAlreadyExists(err)
		}, func() error {
			_, err := cs.RbacV1().RoleBindings(roleBinding.Namespace).Create(ctx, &roleBinding, metav1.CreateOptions{})
			return err
		},
	); err != nil && apierrors.IsAlreadyExists(err) {
		return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			retrievedR, err := cs.RbacV1().RoleBindings(roleBinding.Namespace).Get(ctx, roleBinding.Name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if retrievedR.Annotations == nil {
				retrievedR.Annotations = make(map[string]string, len(roleBinding.Annotations))
			}
			for k, v := range roleBinding.Annotations {
				retrievedR.Annotations[k] = v
			}
			retrievedR.Subjects = roleBinding.Subjects
			retrievedR.RoleRef = roleBinding.RoleRef
			_, err = cs.RbacV1().RoleBindings(roleBinding.Namespace).Update(ctx, retrievedR, metav1.UpdateOptions{})
			return err
		})
	} else if err != nil {
		return err
	}
	return nil
}
