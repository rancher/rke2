/*
Copyright 2020 The Kubernetes Authors.

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

package storageversiongc

import (
	"context"
	"fmt"
	"time"

	apiserverinternalv1alpha1 "k8s.io/api/apiserverinternal/v1alpha1"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/storageversion"
	apiserverinternalinformers "k8s.io/client-go/informers/apiserverinternal/v1alpha1"
	coordinformers "k8s.io/client-go/informers/coordination/v1"
	"k8s.io/client-go/kubernetes"
	coordlisters "k8s.io/client-go/listers/coordination/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controlplane"

	"k8s.io/klog/v2"
)

// Controller watches kube-apiserver leases and storageversions, and delete stale
// storage version entries and objects.
type Controller struct {
	kubeclientset kubernetes.Interface

	leaseLister  coordlisters.LeaseLister
	leasesSynced cache.InformerSynced

	storageVersionSynced cache.InformerSynced

	leaseQueue          workqueue.RateLimitingInterface
	storageVersionQueue workqueue.RateLimitingInterface
}

// NewStorageVersionGC creates a new Controller.
func NewStorageVersionGC(clientset kubernetes.Interface, leaseInformer coordinformers.LeaseInformer, storageVersionInformer apiserverinternalinformers.StorageVersionInformer) *Controller {
	c := &Controller{
		kubeclientset:        clientset,
		leaseLister:          leaseInformer.Lister(),
		leasesSynced:         leaseInformer.Informer().HasSynced,
		storageVersionSynced: storageVersionInformer.Informer().HasSynced,
		leaseQueue:           workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "storage_version_garbage_collector_leases"),
		storageVersionQueue:  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "storage_version_garbage_collector_storageversions"),
	}

	leaseInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: c.onDeleteLease,
	})
	// use the default resync period from the informer
	storageVersionInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.onAddStorageVersion,
		UpdateFunc: c.onUpdateStorageVersion,
	})

	return c
}

// Run starts one worker.
func (c *Controller) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.leaseQueue.ShutDown()
	defer c.storageVersionQueue.ShutDown()
	defer klog.Infof("Shutting down storage version garbage collector")

	klog.Infof("Starting storage version garbage collector")

	if !cache.WaitForCacheSync(stopCh, c.leasesSynced, c.storageVersionSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	// Identity lease deletion and storageversion update don't happen too often. Start one
	// worker for each of them.
	// runLeaseWorker handles legit identity lease deletion, while runStorageVersionWorker
	// handles storageversion creation/update with non-existing id. The latter should rarely
	// happen. It's okay for the two workers to conflict on update.
	go wait.Until(c.runLeaseWorker, time.Second, stopCh)
	go wait.Until(c.runStorageVersionWorker, time.Second, stopCh)

	<-stopCh
}

func (c *Controller) runLeaseWorker() {
	for c.processNextLease() {
	}
}

func (c *Controller) processNextLease() bool {
	key, quit := c.leaseQueue.Get()
	if quit {
		return false
	}
	defer c.leaseQueue.Done(key)

	err := c.processDeletedLease(key.(string))
	if err == nil {
		c.leaseQueue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("lease %v failed with: %v", key, err))
	c.leaseQueue.AddRateLimited(key)
	return true
}

func (c *Controller) runStorageVersionWorker() {
	for c.processNextStorageVersion() {
	}
}

func (c *Controller) processNextStorageVersion() bool {
	key, quit := c.storageVersionQueue.Get()
	if quit {
		return false
	}
	defer c.storageVersionQueue.Done(key)

	err := c.syncStorageVersion(key.(string))
	if err == nil {
		c.storageVersionQueue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("storage version %v failed with: %v", key, err))
	c.storageVersionQueue.AddRateLimited(key)
	return true
}

func (c *Controller) processDeletedLease(name string) error {
	_, err := c.kubeclientset.CoordinationV1().Leases(metav1.NamespaceSystem).Get(context.TODO(), name, metav1.GetOptions{})
	// the lease isn't deleted, nothing we need to do here
	if err == nil {
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return err
	}
	// the frequency of this call won't be too high because we only trigger on identity lease deletions
	storageVersionList, err := c.kubeclientset.InternalV1alpha1().StorageVersions().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}

	var errors []error
	for _, sv := range storageVersionList.Items {
		var serverStorageVersions []apiserverinternalv1alpha1.ServerStorageVersion
		hasStaleRecord := false
		for _, ssv := range sv.Status.StorageVersions {
			if ssv.APIServerID == name {
				hasStaleRecord = true
				continue
			}
			serverStorageVersions = append(serverStorageVersions, ssv)
		}
		if !hasStaleRecord {
			continue
		}
		if err := c.updateOrDeleteStorageVersion(&sv, serverStorageVersions); err != nil {
			errors = append(errors, err)
		}
	}

	return utilerrors.NewAggregate(errors)
}

func (c *Controller) syncStorageVersion(name string) error {
	sv, err := c.kubeclientset.InternalV1alpha1().StorageVersions().Get(context.TODO(), name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		// The problematic storage version that was added/updated recently is gone.
		// Nothing we need to do here.
		return nil
	}
	if err != nil {
		return err
	}

	hasInvalidID := false
	var serverStorageVersions []apiserverinternalv1alpha1.ServerStorageVersion
	for _, v := range sv.Status.StorageVersions {
		lease, err := c.kubeclientset.CoordinationV1().Leases(metav1.NamespaceSystem).Get(context.TODO(), v.APIServerID, metav1.GetOptions{})
		if err != nil || lease == nil || lease.Labels == nil ||
			lease.Labels[controlplane.IdentityLeaseComponentLabelKey] != controlplane.KubeAPIServer {
			// We cannot find a corresponding identity lease from apiserver as well.
			// We need to clean up this storage version.
			hasInvalidID = true
			continue
		}
		serverStorageVersions = append(serverStorageVersions, v)
	}
	if !hasInvalidID {
		return nil
	}
	return c.updateOrDeleteStorageVersion(sv, serverStorageVersions)
}

func (c *Controller) onAddStorageVersion(obj interface{}) {
	castObj := obj.(*apiserverinternalv1alpha1.StorageVersion)
	c.enqueueStorageVersion(castObj)
}

func (c *Controller) onUpdateStorageVersion(oldObj, newObj interface{}) {
	castNewObj := newObj.(*apiserverinternalv1alpha1.StorageVersion)
	c.enqueueStorageVersion(castNewObj)
}

// enqueueStorageVersion enqueues the storage version if it has entry for invalid apiserver
func (c *Controller) enqueueStorageVersion(obj *apiserverinternalv1alpha1.StorageVersion) {
	for _, sv := range obj.Status.StorageVersions {
		lease, err := c.leaseLister.Leases(metav1.NamespaceSystem).Get(sv.APIServerID)
		if err != nil || lease == nil || lease.Labels == nil ||
			lease.Labels[controlplane.IdentityLeaseComponentLabelKey] != controlplane.KubeAPIServer {
			// we cannot find a corresponding identity lease in cache, enqueue the storageversion
			klog.V(4).Infof("Observed storage version %s with invalid apiserver entry", obj.Name)
			c.storageVersionQueue.Add(obj.Name)
			return
		}
	}
}

func (c *Controller) onDeleteLease(obj interface{}) {
	castObj, ok := obj.(*coordinationv1.Lease)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		castObj, ok = tombstone.Obj.(*coordinationv1.Lease)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Lease %#v", obj))
			return
		}
	}

	if castObj.Namespace == metav1.NamespaceSystem &&
		castObj.Labels != nil &&
		castObj.Labels[controlplane.IdentityLeaseComponentLabelKey] == controlplane.KubeAPIServer {
		klog.V(4).Infof("Observed lease %s deleted", castObj.Name)
		c.enqueueLease(castObj)
	}
}

func (c *Controller) enqueueLease(obj *coordinationv1.Lease) {
	c.leaseQueue.Add(obj.Name)
}

func (c *Controller) updateOrDeleteStorageVersion(sv *apiserverinternalv1alpha1.StorageVersion, serverStorageVersions []apiserverinternalv1alpha1.ServerStorageVersion) error {
	if len(serverStorageVersions) == 0 {
		return c.kubeclientset.InternalV1alpha1().StorageVersions().Delete(
			context.TODO(), sv.Name, metav1.DeleteOptions{})
	}
	sv.Status.StorageVersions = serverStorageVersions
	storageversion.SetCommonEncodingVersion(sv)
	_, err := c.kubeclientset.InternalV1alpha1().StorageVersions().UpdateStatus(
		context.TODO(), sv, metav1.UpdateOptions{})
	return err
}
