/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rest

import (
	"context"
	"fmt"
	"time"

	flowcontrolv1beta1 "k8s.io/api/flowcontrol/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	flowcontrolbootstrap "k8s.io/apiserver/pkg/apis/flowcontrol/bootstrap"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	serverstorage "k8s.io/apiserver/pkg/server/storage"
	flowcontrolclient "k8s.io/client-go/kubernetes/typed/flowcontrol/v1beta1"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/flowcontrol"
	flowcontrolapisv1alpha1 "k8s.io/kubernetes/pkg/apis/flowcontrol/v1alpha1"
	flowcontrolapisv1beta1 "k8s.io/kubernetes/pkg/apis/flowcontrol/v1beta1"
	flowschemastore "k8s.io/kubernetes/pkg/registry/flowcontrol/flowschema/storage"
	prioritylevelconfigurationstore "k8s.io/kubernetes/pkg/registry/flowcontrol/prioritylevelconfiguration/storage"
)

var _ genericapiserver.PostStartHookProvider = RESTStorageProvider{}

// RESTStorageProvider is a provider of REST storage
type RESTStorageProvider struct{}

// PostStartHookName is the name of the post-start-hook provided by flow-control storage
const PostStartHookName = "priority-and-fairness-config-producer"

// NewRESTStorage creates a new rest storage for flow-control api models.
func (p RESTStorageProvider) NewRESTStorage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) (genericapiserver.APIGroupInfo, bool, error) {
	apiGroupInfo := genericapiserver.NewDefaultAPIGroupInfo(flowcontrol.GroupName, legacyscheme.Scheme, legacyscheme.ParameterCodec, legacyscheme.Codecs)

	if apiResourceConfigSource.VersionEnabled(flowcontrolapisv1alpha1.SchemeGroupVersion) {
		flowControlStorage, err := p.storage(apiResourceConfigSource, restOptionsGetter)
		if err != nil {
			return genericapiserver.APIGroupInfo{}, false, err
		}
		apiGroupInfo.VersionedResourcesStorageMap[flowcontrolapisv1alpha1.SchemeGroupVersion.Version] = flowControlStorage
	}

	if apiResourceConfigSource.VersionEnabled(flowcontrolapisv1beta1.SchemeGroupVersion) {
		flowControlStorage, err := p.storage(apiResourceConfigSource, restOptionsGetter)
		if err != nil {
			return genericapiserver.APIGroupInfo{}, false, err
		}
		apiGroupInfo.VersionedResourcesStorageMap[flowcontrolapisv1beta1.SchemeGroupVersion.Version] = flowControlStorage
	}

	return apiGroupInfo, true, nil
}

func (p RESTStorageProvider) storage(apiResourceConfigSource serverstorage.APIResourceConfigSource, restOptionsGetter generic.RESTOptionsGetter) (map[string]rest.Storage, error) {
	storage := map[string]rest.Storage{}

	// flow-schema
	flowSchemaStorage, flowSchemaStatusStorage, err := flowschemastore.NewREST(restOptionsGetter)
	if err != nil {
		return nil, err
	}
	storage["flowschemas"] = flowSchemaStorage
	storage["flowschemas/status"] = flowSchemaStatusStorage

	// priority-level-configuration
	priorityLevelConfigurationStorage, priorityLevelConfigurationStatusStorage, err := prioritylevelconfigurationstore.NewREST(restOptionsGetter)
	if err != nil {
		return nil, err
	}
	storage["prioritylevelconfigurations"] = priorityLevelConfigurationStorage
	storage["prioritylevelconfigurations/status"] = priorityLevelConfigurationStatusStorage

	return storage, nil
}

// GroupName returns group name of the storage
func (p RESTStorageProvider) GroupName() string {
	return flowcontrol.GroupName
}

// PostStartHook returns the hook func that launches the config provider
func (p RESTStorageProvider) PostStartHook() (string, genericapiserver.PostStartHookFunc, error) {
	return PostStartHookName, func(hookContext genericapiserver.PostStartHookContext) error {
		flowcontrolClientSet := flowcontrolclient.NewForConfigOrDie(hookContext.LoopbackClientConfig)
		go func() {
			const retryCreatingSuggestedSettingsInterval = time.Second
			err := wait.PollImmediateUntil(
				retryCreatingSuggestedSettingsInterval,
				func() (bool, error) {
					should, err := shouldEnsureSuggested(flowcontrolClientSet)
					if err != nil {
						klog.Errorf("failed getting exempt flow-schema, will retry later: %v", err)
						return false, nil
					}
					if !should {
						return true, nil
					}
					err = ensure(
						flowcontrolClientSet,
						flowcontrolbootstrap.SuggestedFlowSchemas,
						flowcontrolbootstrap.SuggestedPriorityLevelConfigurations)
					if err != nil {
						klog.Errorf("failed ensuring suggested settings, will retry later: %v", err)
						return false, nil
					}
					return true, nil
				},
				hookContext.StopCh)
			if err != nil {
				klog.ErrorS(err, "Ensuring suggested configuration failed")

				// We should not attempt creation of mandatory objects if ensuring the suggested
				// configuration resulted in an error.
				// This only happens when the stop channel is closed.
				// We rely on the presence of the "exempt" priority level configuration object in the cluster
				// to indicate whether we should ensure suggested configuration.
				return
			}

			const retryCreatingMandatorySettingsInterval = time.Minute
			_ = wait.PollImmediateUntil(
				retryCreatingMandatorySettingsInterval,
				func() (bool, error) {
					if err := upgrade(
						flowcontrolClientSet,
						flowcontrolbootstrap.MandatoryFlowSchemas,
						// Note: the "exempt" priority-level is supposed to be the last item in the pre-defined
						// list, so that a crash in the midst of the first kube-apiserver startup does not prevent
						// the full initial set of objects from being created.
						flowcontrolbootstrap.MandatoryPriorityLevelConfigurations,
					); err != nil {
						klog.Errorf("failed creating mandatory flowcontrol settings: %v", err)
						return false, nil
					}
					return false, nil // always retry
				},
				hookContext.StopCh)
		}()
		return nil
	}, nil

}

// shouldEnsureSuggested checks if the exempt priority level exists and returns
// whether the suggested flow schemas and priority levels should be ensured.
func shouldEnsureSuggested(flowcontrolClientSet flowcontrolclient.FlowcontrolV1beta1Interface) (bool, error) {
	if _, err := flowcontrolClientSet.PriorityLevelConfigurations().Get(context.TODO(), flowcontrol.PriorityLevelConfigurationNameExempt, metav1.GetOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}
		return false, err
	}
	return false, nil
}

const thisFieldManager = "api-priority-and-fairness-config-producer-v1"

func ensure(flowcontrolClientSet flowcontrolclient.FlowcontrolV1beta1Interface, flowSchemas []*flowcontrolv1beta1.FlowSchema, priorityLevels []*flowcontrolv1beta1.PriorityLevelConfiguration) error {
	for _, flowSchema := range flowSchemas {
		_, err := flowcontrolClientSet.FlowSchemas().Create(context.TODO(), flowSchema, metav1.CreateOptions{FieldManager: thisFieldManager})
		if apierrors.IsAlreadyExists(err) {
			klog.V(3).Infof("Suggested FlowSchema %s already exists, skipping creating", flowSchema.Name)
			continue
		}
		if err != nil {
			return fmt.Errorf("cannot create suggested FlowSchema %s due to %v", flowSchema.Name, err)
		}
		klog.V(3).Infof("Created suggested FlowSchema %s", flowSchema.Name)
	}
	for _, priorityLevelConfiguration := range priorityLevels {
		_, err := flowcontrolClientSet.PriorityLevelConfigurations().Create(context.TODO(), priorityLevelConfiguration, metav1.CreateOptions{FieldManager: thisFieldManager})
		if apierrors.IsAlreadyExists(err) {
			klog.V(3).Infof("Suggested PriorityLevelConfiguration %s already exists, skipping creating", priorityLevelConfiguration.Name)
			continue
		}
		if err != nil {
			return fmt.Errorf("cannot create suggested PriorityLevelConfiguration %s due to %v", priorityLevelConfiguration.Name, err)
		}
		klog.V(3).Infof("Created suggested PriorityLevelConfiguration %s", priorityLevelConfiguration.Name)
	}
	return nil
}

func upgrade(flowcontrolClientSet flowcontrolclient.FlowcontrolV1beta1Interface, flowSchemas []*flowcontrolv1beta1.FlowSchema, priorityLevels []*flowcontrolv1beta1.PriorityLevelConfiguration) error {
	for _, expectedFlowSchema := range flowSchemas {
		actualFlowSchema, err := flowcontrolClientSet.FlowSchemas().Get(context.TODO(), expectedFlowSchema.Name, metav1.GetOptions{})
		if err == nil {
			// TODO(yue9944882): extract existing version from label and compare
			// TODO(yue9944882): create w/ version string attached
			wrongSpec, err := flowSchemaHasWrongSpec(expectedFlowSchema, actualFlowSchema)
			if err != nil {
				return fmt.Errorf("failed checking if mandatory FlowSchema %s is up-to-date due to %v, will retry later", expectedFlowSchema.Name, err)
			}
			if wrongSpec {
				if _, err := flowcontrolClientSet.FlowSchemas().Update(context.TODO(), expectedFlowSchema, metav1.UpdateOptions{FieldManager: thisFieldManager}); err != nil {
					return fmt.Errorf("failed upgrading mandatory FlowSchema %s due to %v, will retry later", expectedFlowSchema.Name, err)
				}
				klog.V(3).Infof("Updated mandatory FlowSchema %s because its spec was %#+v but it must be %#+v", expectedFlowSchema.Name, actualFlowSchema.Spec, expectedFlowSchema.Spec)
			}
			continue
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed getting mandatory FlowSchema %s due to %v, will retry later", expectedFlowSchema.Name, err)
		}
		_, err = flowcontrolClientSet.FlowSchemas().Create(context.TODO(), expectedFlowSchema, metav1.CreateOptions{FieldManager: thisFieldManager})
		if apierrors.IsAlreadyExists(err) {
			klog.V(3).Infof("Mandatory FlowSchema %s already exists, skipping creating", expectedFlowSchema.Name)
			continue
		}
		if err != nil {
			return fmt.Errorf("cannot create mandatory FlowSchema %s due to %v", expectedFlowSchema.Name, err)
		}
		klog.V(3).Infof("Created mandatory FlowSchema %s", expectedFlowSchema.Name)
	}
	for _, expectedPriorityLevelConfiguration := range priorityLevels {
		actualPriorityLevelConfiguration, err := flowcontrolClientSet.PriorityLevelConfigurations().Get(context.TODO(), expectedPriorityLevelConfiguration.Name, metav1.GetOptions{})
		if err == nil {
			// TODO(yue9944882): extract existing version from label and compare
			// TODO(yue9944882): create w/ version string attached
			wrongSpec, err := priorityLevelHasWrongSpec(expectedPriorityLevelConfiguration, actualPriorityLevelConfiguration)
			if err != nil {
				return fmt.Errorf("failed checking if mandatory PriorityLevelConfiguration %s is up-to-date due to %v, will retry later", expectedPriorityLevelConfiguration.Name, err)
			}
			if wrongSpec {
				if _, err := flowcontrolClientSet.PriorityLevelConfigurations().Update(context.TODO(), expectedPriorityLevelConfiguration, metav1.UpdateOptions{FieldManager: thisFieldManager}); err != nil {
					return fmt.Errorf("failed upgrading mandatory PriorityLevelConfiguration %s due to %v, will retry later", expectedPriorityLevelConfiguration.Name, err)
				}
				klog.V(3).Infof("Updated mandatory PriorityLevelConfiguration %s because its spec was %#+v but must be %#+v", expectedPriorityLevelConfiguration.Name, actualPriorityLevelConfiguration.Spec, expectedPriorityLevelConfiguration.Spec)
			}
			continue
		}
		if !apierrors.IsNotFound(err) {
			return fmt.Errorf("failed getting PriorityLevelConfiguration %s due to %v, will retry later", expectedPriorityLevelConfiguration.Name, err)
		}
		_, err = flowcontrolClientSet.PriorityLevelConfigurations().Create(context.TODO(), expectedPriorityLevelConfiguration, metav1.CreateOptions{FieldManager: thisFieldManager})
		if apierrors.IsAlreadyExists(err) {
			klog.V(3).Infof("Mandatory PriorityLevelConfiguration %s already exists, skipping creating", expectedPriorityLevelConfiguration.Name)
			continue
		}
		if err != nil {
			return fmt.Errorf("cannot create mandatory PriorityLevelConfiguration %s due to %v", expectedPriorityLevelConfiguration.Name, err)
		}
		klog.V(3).Infof("Created mandatory PriorityLevelConfiguration %s", expectedPriorityLevelConfiguration.Name)
	}
	return nil
}

func flowSchemaHasWrongSpec(expected, actual *flowcontrolv1beta1.FlowSchema) (bool, error) {
	copiedExpectedFlowSchema := expected.DeepCopy()
	flowcontrolapisv1beta1.SetObjectDefaults_FlowSchema(copiedExpectedFlowSchema)
	return !equality.Semantic.DeepEqual(copiedExpectedFlowSchema.Spec, actual.Spec), nil
}

func priorityLevelHasWrongSpec(expected, actual *flowcontrolv1beta1.PriorityLevelConfiguration) (bool, error) {
	copiedExpectedPriorityLevel := expected.DeepCopy()
	flowcontrolapisv1beta1.SetObjectDefaults_PriorityLevelConfiguration(copiedExpectedPriorityLevel)
	return !equality.Semantic.DeepEqual(copiedExpectedPriorityLevel.Spec, actual.Spec), nil
}
