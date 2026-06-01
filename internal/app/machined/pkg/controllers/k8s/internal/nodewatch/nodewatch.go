// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package nodewatch implements Kubernetes node watcher.
package nodewatch

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	informersv1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/tools/cache"

	"github.com/siderolabs/talos/pkg/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// syncTickInterval is how often a "still waiting for initial sync" error is reported to the caller
// while the informer hasn't yet completed its initial list. Each tick increments the caller's
// error counter; once the caller's threshold is reached, the watcher is torn down and rebuilt.
const syncTickInterval = 10 * time.Second

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
//
//nolint:gocyclo
func (r *NodeWatcher) Watch(ctx context.Context, logger *zap.Logger) (<-chan struct{}, <-chan error, func(), error) {
	logger.Debug("starting node watcher", zap.String("nodename", r.nodename))

	informerFactory := informers.NewSharedInformerFactoryWithOptions(
		r.client.Clientset,
		constants.KubernetesInformerDefaultResyncPeriod,
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

	syncCtx, syncCancel := context.WithCancel(ctx)

	go func() {
		logger.Debug("waiting for node cache sync")

		result := informerFactory.WaitForCacheSync(ctx.Done())

		// stop the ticker goroutine below as soon as sync completes (or ctx is done)
		syncCancel()

		var synced bool

		// result should contain a single entry
		for _, v := range result {
			synced = v
		}

		logger.Debug("node cache sync done", zap.Bool("synced", synced))

		select {
		case notifyCh <- struct{}{}:
		default:
		}
	}()

	// While the informer is still performing its initial list/sync, periodically
	// surface a timeout error to the caller's watchErrCh. client-go's reflector
	// silently retries connection-refused errors during the initial list, so the
	// WatchErrorHandler never fires in that scenario. Pushing ticks here lets the
	// caller apply its own threshold and restart the watcher with a fresh client.
	go func() {
		ticker := time.NewTicker(syncTickInterval)
		defer ticker.Stop()

		for {
			select {
			case <-syncCtx.Done():
				return
			case <-ticker.C:
				select {
				case <-syncCtx.Done():
					return
				case watchErrCh <- fmt.Errorf("node cache: no sync for %s", syncTickInterval):
				}
			}
		}
	}()

	return notifyCh, watchErrCh, informerFactory.Shutdown, nil
}
