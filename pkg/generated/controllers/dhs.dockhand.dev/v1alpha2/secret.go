/*
Copyright © 2024 BoxBoat

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
// Code generated by main. DO NOT EDIT.

package v1alpha2

import (
	"context"
	"sync"
	"time"

	v1alpha2 "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dhs.dockhand.dev/v1alpha2"
	"github.com/rancher/lasso/pkg/client"
	"github.com/rancher/lasso/pkg/controller"
	"github.com/rancher/wrangler/pkg/apply"
	"github.com/rancher/wrangler/pkg/condition"
	"github.com/rancher/wrangler/pkg/generic"
	"github.com/rancher/wrangler/pkg/kv"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
)

type SecretHandler func(string, *v1alpha2.Secret) (*v1alpha2.Secret, error)

type SecretController interface {
	generic.ControllerMeta
	SecretClient

	OnChange(ctx context.Context, name string, sync SecretHandler)
	OnRemove(ctx context.Context, name string, sync SecretHandler)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, duration time.Duration)

	Cache() SecretCache
}

type SecretClient interface {
	Create(*v1alpha2.Secret) (*v1alpha2.Secret, error)
	Update(*v1alpha2.Secret) (*v1alpha2.Secret, error)
	UpdateStatus(*v1alpha2.Secret) (*v1alpha2.Secret, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	Get(namespace, name string, options metav1.GetOptions) (*v1alpha2.Secret, error)
	List(namespace string, opts metav1.ListOptions) (*v1alpha2.SecretList, error)
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha2.Secret, err error)
}

type SecretCache interface {
	Get(namespace, name string) (*v1alpha2.Secret, error)
	List(namespace string, selector labels.Selector) ([]*v1alpha2.Secret, error)

	AddIndexer(indexName string, indexer SecretIndexer)
	GetByIndex(indexName, key string) ([]*v1alpha2.Secret, error)
}

type SecretIndexer func(obj *v1alpha2.Secret) ([]string, error)

type secretController struct {
	controller    controller.SharedController
	client        *client.Client
	gvk           schema.GroupVersionKind
	groupResource schema.GroupResource
}

func NewSecretController(gvk schema.GroupVersionKind, resource string, namespaced bool, controller controller.SharedControllerFactory) SecretController {
	c := controller.ForResourceKind(gvk.GroupVersion().WithResource(resource), gvk.Kind, namespaced)
	return &secretController{
		controller: c,
		client:     c.Client(),
		gvk:        gvk,
		groupResource: schema.GroupResource{
			Group:    gvk.Group,
			Resource: resource,
		},
	}
}

func FromSecretHandlerToHandler(sync SecretHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v1alpha2.Secret
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v1alpha2.Secret))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *secretController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v1alpha2.Secret))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateSecretDeepCopyOnChange(client SecretClient, obj *v1alpha2.Secret, handler func(obj *v1alpha2.Secret) (*v1alpha2.Secret, error)) (*v1alpha2.Secret, error) {
	if obj == nil {
		return obj, nil
	}

	copyObj := obj.DeepCopy()
	newObj, err := handler(copyObj)
	if newObj != nil {
		copyObj = newObj
	}
	if obj.ResourceVersion == copyObj.ResourceVersion && !equality.Semantic.DeepEqual(obj, copyObj) {
		return client.Update(copyObj)
	}

	return copyObj, err
}

func (c *secretController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}

func (c *secretController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), handler))
}

func (c *secretController) OnChange(ctx context.Context, name string, sync SecretHandler) {
	c.AddGenericHandler(ctx, name, FromSecretHandlerToHandler(sync))
}

func (c *secretController) OnRemove(ctx context.Context, name string, sync SecretHandler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), FromSecretHandlerToHandler(sync)))
}

func (c *secretController) Enqueue(namespace, name string) {
	c.controller.Enqueue(namespace, name)
}

func (c *secretController) EnqueueAfter(namespace, name string, duration time.Duration) {
	c.controller.EnqueueAfter(namespace, name, duration)
}

func (c *secretController) Informer() cache.SharedIndexInformer {
	return c.controller.Informer()
}

func (c *secretController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *secretController) Cache() SecretCache {
	return &secretCache{
		indexer:  c.Informer().GetIndexer(),
		resource: c.groupResource,
	}
}

func (c *secretController) Create(obj *v1alpha2.Secret) (*v1alpha2.Secret, error) {
	result := &v1alpha2.Secret{}
	return result, c.client.Create(context.TODO(), obj.Namespace, obj, result, metav1.CreateOptions{})
}

func (c *secretController) Update(obj *v1alpha2.Secret) (*v1alpha2.Secret, error) {
	result := &v1alpha2.Secret{}
	return result, c.client.Update(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *secretController) UpdateStatus(obj *v1alpha2.Secret) (*v1alpha2.Secret, error) {
	result := &v1alpha2.Secret{}
	return result, c.client.UpdateStatus(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *secretController) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.client.Delete(context.TODO(), namespace, name, *options)
}

func (c *secretController) Get(namespace, name string, options metav1.GetOptions) (*v1alpha2.Secret, error) {
	result := &v1alpha2.Secret{}
	return result, c.client.Get(context.TODO(), namespace, name, result, options)
}

func (c *secretController) List(namespace string, opts metav1.ListOptions) (*v1alpha2.SecretList, error) {
	result := &v1alpha2.SecretList{}
	return result, c.client.List(context.TODO(), namespace, result, opts)
}

func (c *secretController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.Watch(context.TODO(), namespace, opts)
}

func (c *secretController) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (*v1alpha2.Secret, error) {
	result := &v1alpha2.Secret{}
	return result, c.client.Patch(context.TODO(), namespace, name, pt, data, result, metav1.PatchOptions{}, subresources...)
}

type secretCache struct {
	indexer  cache.Indexer
	resource schema.GroupResource
}

func (c *secretCache) Get(namespace, name string) (*v1alpha2.Secret, error) {
	obj, exists, err := c.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(c.resource, name)
	}
	return obj.(*v1alpha2.Secret), nil
}

func (c *secretCache) List(namespace string, selector labels.Selector) (ret []*v1alpha2.Secret, err error) {

	err = cache.ListAllByNamespace(c.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha2.Secret))
	})

	return ret, err
}

func (c *secretCache) AddIndexer(indexName string, indexer SecretIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v1alpha2.Secret))
		},
	}))
}

func (c *secretCache) GetByIndex(indexName, key string) (result []*v1alpha2.Secret, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*v1alpha2.Secret, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*v1alpha2.Secret))
	}
	return result, nil
}

// SecretStatusHandler is executed for every added or modified Secret. Should return the new status to be updated
type SecretStatusHandler func(obj *v1alpha2.Secret, status v1alpha2.SecretStatus) (v1alpha2.SecretStatus, error)

// SecretGeneratingHandler is the top-level handler that is executed for every Secret event. It extends SecretStatusHandler by a returning a slice of child objects to be passed to apply.Apply
type SecretGeneratingHandler func(obj *v1alpha2.Secret, status v1alpha2.SecretStatus) ([]runtime.Object, v1alpha2.SecretStatus, error)

// RegisterSecretStatusHandler configures a SecretController to execute a SecretStatusHandler for every events observed.
// If a non-empty condition is provided, it will be updated in the status conditions for every handler execution
func RegisterSecretStatusHandler(ctx context.Context, controller SecretController, condition condition.Cond, name string, handler SecretStatusHandler) {
	statusHandler := &secretStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, FromSecretHandlerToHandler(statusHandler.sync))
}

// RegisterSecretGeneratingHandler configures a SecretController to execute a SecretGeneratingHandler for every events observed, passing the returned objects to the provided apply.Apply.
// If a non-empty condition is provided, it will be updated in the status conditions for every handler execution
func RegisterSecretGeneratingHandler(ctx context.Context, controller SecretController, apply apply.Apply,
	condition condition.Cond, name string, handler SecretGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &secretGeneratingHandler{
		SecretGeneratingHandler: handler,
		apply:                   apply,
		name:                    name,
		gvk:                     controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterSecretStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type secretStatusHandler struct {
	client    SecretClient
	condition condition.Cond
	handler   SecretStatusHandler
}

// sync is executed on every resource addition or modification. Executes the configured handlers and sends the updated status to the Kubernetes API
func (a *secretStatusHandler) sync(key string, obj *v1alpha2.Secret) (*v1alpha2.Secret, error) {
	if obj == nil {
		return obj, nil
	}

	origStatus := obj.Status.DeepCopy()
	obj = obj.DeepCopy()
	newStatus, err := a.handler(obj, obj.Status)
	if err != nil {
		// Revert to old status on error
		newStatus = *origStatus.DeepCopy()
	}

	if a.condition != "" {
		if errors.IsConflict(err) {
			a.condition.SetError(&newStatus, "", nil)
		} else {
			a.condition.SetError(&newStatus, "", err)
		}
	}
	if !equality.Semantic.DeepEqual(origStatus, &newStatus) {
		if a.condition != "" {
			// Since status has changed, update the lastUpdatedTime
			a.condition.LastUpdated(&newStatus, time.Now().UTC().Format(time.RFC3339))
		}

		var newErr error
		obj.Status = newStatus
		newObj, newErr := a.client.UpdateStatus(obj)
		if err == nil {
			err = newErr
		}
		if newErr == nil {
			obj = newObj
		}
	}
	return obj, err
}

type secretGeneratingHandler struct {
	SecretGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
	seen  sync.Map
}

// Remove handles the observed deletion of a resource, cascade deleting every associated resource previously applied
func (a *secretGeneratingHandler) Remove(key string, obj *v1alpha2.Secret) (*v1alpha2.Secret, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v1alpha2.Secret{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	if a.opts.UniqueApplyForResourceVersion {
		a.seen.Delete(key)
	}

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

// Handle executes the configured SecretGeneratingHandler and pass the resulting objects to apply.Apply, finally returning the new status of the resource
func (a *secretGeneratingHandler) Handle(obj *v1alpha2.Secret, status v1alpha2.SecretStatus) (v1alpha2.SecretStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.SecretGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}
	if !a.isNewResourceVersion(obj) {
		return newStatus, nil
	}

	err = generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
	if err != nil {
		return newStatus, err
	}
	a.storeResourceVersion(obj)
	return newStatus, nil
}

// isNewResourceVersion detects if a specific resource version was already successfully processed.
// Only used if UniqueApplyForResourceVersion is set in generic.GeneratingHandlerOptions
func (a *secretGeneratingHandler) isNewResourceVersion(obj *v1alpha2.Secret) bool {
	if !a.opts.UniqueApplyForResourceVersion {
		return true
	}

	// Apply once per resource version
	key := obj.Namespace + "/" + obj.Name
	previous, ok := a.seen.Load(key)
	return !ok || previous != obj.ResourceVersion
}

// storeResourceVersion keeps track of the latest resource version of an object for which Apply was executed
// Only used if UniqueApplyForResourceVersion is set in generic.GeneratingHandlerOptions
func (a *secretGeneratingHandler) storeResourceVersion(obj *v1alpha2.Secret) {
	if !a.opts.UniqueApplyForResourceVersion {
		return
	}

	key := obj.Namespace + "/" + obj.Name
	a.seen.Store(key, obj.ResourceVersion)
}
