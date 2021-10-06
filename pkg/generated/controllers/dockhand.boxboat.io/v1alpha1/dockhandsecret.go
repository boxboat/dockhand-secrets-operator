/*
Copyright © 2021 BoxBoat

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

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/boxboat/dockhand-secrets-operator/pkg/apis/dockhand.boxboat.io/v1alpha1"
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

type DockhandSecretHandler func(string, *v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error)

type DockhandSecretController interface {
	generic.ControllerMeta
	DockhandSecretClient

	OnChange(ctx context.Context, name string, sync DockhandSecretHandler)
	OnRemove(ctx context.Context, name string, sync DockhandSecretHandler)
	Enqueue(namespace, name string)
	EnqueueAfter(namespace, name string, duration time.Duration)

	Cache() DockhandSecretCache
}

type DockhandSecretClient interface {
	Create(*v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error)
	Update(*v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error)
	UpdateStatus(*v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error)
	Delete(namespace, name string, options *metav1.DeleteOptions) error
	Get(namespace, name string, options metav1.GetOptions) (*v1alpha1.DockhandSecret, error)
	List(namespace string, opts metav1.ListOptions) (*v1alpha1.DockhandSecretList, error)
	Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error)
	Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (result *v1alpha1.DockhandSecret, err error)
}

type DockhandSecretCache interface {
	Get(namespace, name string) (*v1alpha1.DockhandSecret, error)
	List(namespace string, selector labels.Selector) ([]*v1alpha1.DockhandSecret, error)

	AddIndexer(indexName string, indexer DockhandSecretIndexer)
	GetByIndex(indexName, key string) ([]*v1alpha1.DockhandSecret, error)
}

type DockhandSecretIndexer func(obj *v1alpha1.DockhandSecret) ([]string, error)

type dockhandSecretController struct {
	controller    controller.SharedController
	client        *client.Client
	gvk           schema.GroupVersionKind
	groupResource schema.GroupResource
}

func NewDockhandSecretController(gvk schema.GroupVersionKind, resource string, namespaced bool, controller controller.SharedControllerFactory) DockhandSecretController {
	c := controller.ForResourceKind(gvk.GroupVersion().WithResource(resource), gvk.Kind, namespaced)
	return &dockhandSecretController{
		controller: c,
		client:     c.Client(),
		gvk:        gvk,
		groupResource: schema.GroupResource{
			Group:    gvk.Group,
			Resource: resource,
		},
	}
}

func FromDockhandSecretHandlerToHandler(sync DockhandSecretHandler) generic.Handler {
	return func(key string, obj runtime.Object) (ret runtime.Object, err error) {
		var v *v1alpha1.DockhandSecret
		if obj == nil {
			v, err = sync(key, nil)
		} else {
			v, err = sync(key, obj.(*v1alpha1.DockhandSecret))
		}
		if v == nil {
			return nil, err
		}
		return v, err
	}
}

func (c *dockhandSecretController) Updater() generic.Updater {
	return func(obj runtime.Object) (runtime.Object, error) {
		newObj, err := c.Update(obj.(*v1alpha1.DockhandSecret))
		if newObj == nil {
			return nil, err
		}
		return newObj, err
	}
}

func UpdateDockhandSecretDeepCopyOnChange(client DockhandSecretClient, obj *v1alpha1.DockhandSecret, handler func(obj *v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error)) (*v1alpha1.DockhandSecret, error) {
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

func (c *dockhandSecretController) AddGenericHandler(ctx context.Context, name string, handler generic.Handler) {
	c.controller.RegisterHandler(ctx, name, controller.SharedControllerHandlerFunc(handler))
}

func (c *dockhandSecretController) AddGenericRemoveHandler(ctx context.Context, name string, handler generic.Handler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), handler))
}

func (c *dockhandSecretController) OnChange(ctx context.Context, name string, sync DockhandSecretHandler) {
	c.AddGenericHandler(ctx, name, FromDockhandSecretHandlerToHandler(sync))
}

func (c *dockhandSecretController) OnRemove(ctx context.Context, name string, sync DockhandSecretHandler) {
	c.AddGenericHandler(ctx, name, generic.NewRemoveHandler(name, c.Updater(), FromDockhandSecretHandlerToHandler(sync)))
}

func (c *dockhandSecretController) Enqueue(namespace, name string) {
	c.controller.Enqueue(namespace, name)
}

func (c *dockhandSecretController) EnqueueAfter(namespace, name string, duration time.Duration) {
	c.controller.EnqueueAfter(namespace, name, duration)
}

func (c *dockhandSecretController) Informer() cache.SharedIndexInformer {
	return c.controller.Informer()
}

func (c *dockhandSecretController) GroupVersionKind() schema.GroupVersionKind {
	return c.gvk
}

func (c *dockhandSecretController) Cache() DockhandSecretCache {
	return &dockhandSecretCache{
		indexer:  c.Informer().GetIndexer(),
		resource: c.groupResource,
	}
}

func (c *dockhandSecretController) Create(obj *v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error) {
	result := &v1alpha1.DockhandSecret{}
	return result, c.client.Create(context.TODO(), obj.Namespace, obj, result, metav1.CreateOptions{})
}

func (c *dockhandSecretController) Update(obj *v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error) {
	result := &v1alpha1.DockhandSecret{}
	return result, c.client.Update(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *dockhandSecretController) UpdateStatus(obj *v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error) {
	result := &v1alpha1.DockhandSecret{}
	return result, c.client.UpdateStatus(context.TODO(), obj.Namespace, obj, result, metav1.UpdateOptions{})
}

func (c *dockhandSecretController) Delete(namespace, name string, options *metav1.DeleteOptions) error {
	if options == nil {
		options = &metav1.DeleteOptions{}
	}
	return c.client.Delete(context.TODO(), namespace, name, *options)
}

func (c *dockhandSecretController) Get(namespace, name string, options metav1.GetOptions) (*v1alpha1.DockhandSecret, error) {
	result := &v1alpha1.DockhandSecret{}
	return result, c.client.Get(context.TODO(), namespace, name, result, options)
}

func (c *dockhandSecretController) List(namespace string, opts metav1.ListOptions) (*v1alpha1.DockhandSecretList, error) {
	result := &v1alpha1.DockhandSecretList{}
	return result, c.client.List(context.TODO(), namespace, result, opts)
}

func (c *dockhandSecretController) Watch(namespace string, opts metav1.ListOptions) (watch.Interface, error) {
	return c.client.Watch(context.TODO(), namespace, opts)
}

func (c *dockhandSecretController) Patch(namespace, name string, pt types.PatchType, data []byte, subresources ...string) (*v1alpha1.DockhandSecret, error) {
	result := &v1alpha1.DockhandSecret{}
	return result, c.client.Patch(context.TODO(), namespace, name, pt, data, result, metav1.PatchOptions{}, subresources...)
}

type dockhandSecretCache struct {
	indexer  cache.Indexer
	resource schema.GroupResource
}

func (c *dockhandSecretCache) Get(namespace, name string) (*v1alpha1.DockhandSecret, error) {
	obj, exists, err := c.indexer.GetByKey(namespace + "/" + name)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(c.resource, name)
	}
	return obj.(*v1alpha1.DockhandSecret), nil
}

func (c *dockhandSecretCache) List(namespace string, selector labels.Selector) (ret []*v1alpha1.DockhandSecret, err error) {

	err = cache.ListAllByNamespace(c.indexer, namespace, selector, func(m interface{}) {
		ret = append(ret, m.(*v1alpha1.DockhandSecret))
	})

	return ret, err
}

func (c *dockhandSecretCache) AddIndexer(indexName string, indexer DockhandSecretIndexer) {
	utilruntime.Must(c.indexer.AddIndexers(map[string]cache.IndexFunc{
		indexName: func(obj interface{}) (strings []string, e error) {
			return indexer(obj.(*v1alpha1.DockhandSecret))
		},
	}))
}

func (c *dockhandSecretCache) GetByIndex(indexName, key string) (result []*v1alpha1.DockhandSecret, err error) {
	objs, err := c.indexer.ByIndex(indexName, key)
	if err != nil {
		return nil, err
	}
	result = make([]*v1alpha1.DockhandSecret, 0, len(objs))
	for _, obj := range objs {
		result = append(result, obj.(*v1alpha1.DockhandSecret))
	}
	return result, nil
}

type DockhandSecretStatusHandler func(obj *v1alpha1.DockhandSecret, status v1alpha1.DockhandSecretStatus) (v1alpha1.DockhandSecretStatus, error)

type DockhandSecretGeneratingHandler func(obj *v1alpha1.DockhandSecret, status v1alpha1.DockhandSecretStatus) ([]runtime.Object, v1alpha1.DockhandSecretStatus, error)

func RegisterDockhandSecretStatusHandler(ctx context.Context, controller DockhandSecretController, condition condition.Cond, name string, handler DockhandSecretStatusHandler) {
	statusHandler := &dockhandSecretStatusHandler{
		client:    controller,
		condition: condition,
		handler:   handler,
	}
	controller.AddGenericHandler(ctx, name, FromDockhandSecretHandlerToHandler(statusHandler.sync))
}

func RegisterDockhandSecretGeneratingHandler(ctx context.Context, controller DockhandSecretController, apply apply.Apply,
	condition condition.Cond, name string, handler DockhandSecretGeneratingHandler, opts *generic.GeneratingHandlerOptions) {
	statusHandler := &dockhandSecretGeneratingHandler{
		DockhandSecretGeneratingHandler: handler,
		apply:                           apply,
		name:                            name,
		gvk:                             controller.GroupVersionKind(),
	}
	if opts != nil {
		statusHandler.opts = *opts
	}
	controller.OnChange(ctx, name, statusHandler.Remove)
	RegisterDockhandSecretStatusHandler(ctx, controller, condition, name, statusHandler.Handle)
}

type dockhandSecretStatusHandler struct {
	client    DockhandSecretClient
	condition condition.Cond
	handler   DockhandSecretStatusHandler
}

func (a *dockhandSecretStatusHandler) sync(key string, obj *v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error) {
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

type dockhandSecretGeneratingHandler struct {
	DockhandSecretGeneratingHandler
	apply apply.Apply
	opts  generic.GeneratingHandlerOptions
	gvk   schema.GroupVersionKind
	name  string
}

func (a *dockhandSecretGeneratingHandler) Remove(key string, obj *v1alpha1.DockhandSecret) (*v1alpha1.DockhandSecret, error) {
	if obj != nil {
		return obj, nil
	}

	obj = &v1alpha1.DockhandSecret{}
	obj.Namespace, obj.Name = kv.RSplit(key, "/")
	obj.SetGroupVersionKind(a.gvk)

	return nil, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects()
}

func (a *dockhandSecretGeneratingHandler) Handle(obj *v1alpha1.DockhandSecret, status v1alpha1.DockhandSecretStatus) (v1alpha1.DockhandSecretStatus, error) {
	if !obj.DeletionTimestamp.IsZero() {
		return status, nil
	}

	objs, newStatus, err := a.DockhandSecretGeneratingHandler(obj, status)
	if err != nil {
		return newStatus, err
	}

	return newStatus, generic.ConfigureApplyForObject(a.apply, obj, &a.opts).
		WithOwner(obj).
		WithSetID(a.name).
		ApplyObjects(objs...)
}
