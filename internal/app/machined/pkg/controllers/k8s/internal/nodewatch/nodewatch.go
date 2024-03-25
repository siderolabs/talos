// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nodewatch implements Kubernetes node watcher.
package nodewatch

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/siderolabs/talos/pkg/kubernetes"
)

// NodeWatcher defines a NodeWatcher-based node watcher.
type NodeWatcher struct {
	client *kubernetes.Client

	nodename string
	nodes    informersv1.NodeInformer
}

// NewNodeWatcher creates new Kubernetes node watcher.
func NewNodeWatcher(client *kubernetes.Client, nodename string) *NodeWatcher {
	return &NodeWatcher{
		nodename: nodename,
		client:   client,
	}
}

// Nodename returns the watched nodename.
func (r *NodeWatcher) Nodename() string {
	return r.nodename
}

// Get returns the Node resource.
func (r *NodeWatcher) Get() (*corev1.Node, error) {
	return r.nodes.Lister().Get(r.nodename)
}

// Watch starts watching Node state and notifies on updates via notify channel.
func (r *NodeWatcher) Watch(ctx context.Context) (<-chan struct{}, <-chan error, func(), error) {
	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		r.client.Clientset,
		0,
		informers.WithTweakListOptions(
			func(opts *metav1.ListOptions) {
				opts.FieldSelector = fields.OneTermEqualSelector(metav1.ObjectNameField, r.nodename).String()
			},
		),
	)

	notifyCh := make(chan struct{}, 1)
	watchErrCh := make(chan error, 1)

	notify := func(_ any) {
		select {
		case notifyCh <- struct{}{}:
		default:
		}
	}

	r.nodes = informerFactory.Core().V1().Nodes()

	if err := r.nodes.Informer().SetWatchErrorHandler(func(r *cache.Reflector, err error) {
		select {
		case watchErrCh <- err:
		default:
		}
	}); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to set watch error handler: %w", err)
	}

	if _, err := r.nodes.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    notify,
		DeleteFunc: notify,
		UpdateFunc: func(_, _ any) { notify(nil) },
	}); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to add event handler: %w", err)
	}

	informerFactory.Start(ctx.Done())

	informerFactory.WaitForCacheSync(ctx.Done())

	return notifyCh, watchErrCh, informerFactory.Shutdown, nil
}
