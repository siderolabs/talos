// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"context"
	"fmt"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	drainpkg "github.com/siderolabs/talos/cmd/talosctl/cmd/talos/drain"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/global"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/kubeclient"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/nodedrain"
	"github.com/siderolabs/talos/pkg/reporter"
)

// nodeUpdate carries a progress update from a per-node goroutine to the
// aggregator goroutine that owns the ProgressWriter + Reporter.
type nodeUpdate struct {
	node   string
	update reporter.Update
}

// drainNodes runs Phase 1: resolves the Kubernetes node name for each Talos node
// and performs cordon + drain on all of them in parallel.
//
// It returns a map of talosIP -> k8sNodeName for use in the uncordon phase. The
// map is returned on error too: each name is recorded before that node is
// cordoned, so it holds every node that may have been cordoned before the
// failure and the caller can restore them.
func drainNodes(ctx context.Context, clientFactory *global.ClientFactory, drainTimeout time.Duration, rep *reporter.Reporter) (map[string]string, error) {
	// For kubeconfig - build a random endpoint client (to go to the controlplane).
	c, err := clientFactory.BuildRandomEndpointClient(ctx)
	if err != nil {
		return nil, err
	}

	// Fetch kubeconfig once - it is cluster-global, not node-specific.
	clientset, err := kubeclient.FromTalosClient(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes client for drain: %w", err)
	}

	// Channel for per-node progress updates -> single aggregator goroutine.
	updateCh := make(chan nodeUpdate)

	// k8sNames collects Talos IP -> K8s node name mappings produced by each goroutine.
	k8sNames := make(map[string]string, len(clientFactory.Nodes()))

	var mapMux sync.Mutex // protects k8sNames map during writes

	var eg errgroup.Group

	// Aggregator goroutine: reads from updateCh, updates ProgressWriter, prints.
	// It exits when updateCh is closed (after all workers finish).
	aggregatorDone := make(chan struct{})

	go func() {
		defer close(aggregatorDone)

		var w drainpkg.ProgressWriter

		for upd := range updateCh {
			w.UpdateNode(upd.node, upd.update.Message, upd.update.Status)
			w.PrintProgress(rep)
		}
	}()

	// Launch a goroutine per node.
	for _, node := range clientFactory.Nodes() {
		eg.Go(func() error {
			ctx, c, err := clientFactory.BuildClient(ctx, node)
			if err != nil {
				return fmt.Errorf("error building client for node %s: %w", node, err)
			}

			k8sNodeName, resolveErr := nodedrain.GetKubernetesNodeName(ctx, c)
			if resolveErr != nil {
				return fmt.Errorf("error resolving Kubernetes node name for %s: %w", node, resolveErr)
			}

			mapMux.Lock()
			k8sNames[node] = k8sNodeName
			mapMux.Unlock()

			// reportFn sends progress through the channel to the aggregator.
			reportFn := func(upd reporter.Update) {
				updateCh <- nodeUpdate{node: k8sNodeName, update: upd}
			}

			return nodedrain.CordonAndDrain(ctx, clientset, k8sNodeName, nodedrain.Options{
				DrainTimeout: drainTimeout,
			}, reportFn)
		})
	}

	err = eg.Wait()

	close(updateCh)

	<-aggregatorDone

	if err != nil {
		return k8sNames, err
	}

	return k8sNames, nil
}

// uncordonAbortTimeout bounds the uncordon that follows a failed drain. That path
// runs detached from the caller's context, so it needs a ceiling of its own. The
// budget is dominated by fetching the admin kubeconfig over the Talos API rather
// than by the get and the at-most-one patch per node.
const uncordonAbortTimeout = time.Minute

// uncordonNodes runs Phase 3: waits for each node recorded by drainNodes to become
// Ready, then uncordons them in parallel.
//
// nodeNames maps talosIP -> k8sNodeName (produced by drainNodes).
//
// afterFailedDrain marks the abort path, where nothing was rebooted and so there
// is no readiness to wait for. Waiting anyway would poll any node that is not
// Ready for the full timeout and then leave it cordoned. That path also detaches
// from ctx, which is already canceled whenever the drain failed because the
// operator interrupted it.
func uncordonNodes(
	ctx context.Context, clientFactory *global.ClientFactory, nodeNames map[string]string,
	timeout time.Duration, afterFailedDrain bool, rep *reporter.Reporter,
) error {
	if afterFailedDrain {
		var cancel context.CancelFunc

		ctx, cancel = context.WithTimeout(context.WithoutCancel(ctx), uncordonAbortTimeout)
		defer cancel()
	}

	// For kubeconfig - build a random endpoint client (to go to the controlplane).
	c, err := clientFactory.BuildRandomEndpointClient(ctx)
	if err != nil {
		return err
	}

	// Fetch a fresh kubeconfig (the previous connection may be stale after reboot).
	// The context has no "nodes" metadata (called from WithClientAndNodes), so the
	// request routes to the endpoint which is a control-plane node by convention.
	clientset, err := kubeclient.FromTalosClient(ctx, c)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client for uncordon: %w", err)
	}

	updateCh := make(chan nodeUpdate)

	var eg errgroup.Group

	aggregatorDone := make(chan struct{})

	go func() {
		defer close(aggregatorDone)

		var w drainpkg.ProgressWriter

		for upd := range updateCh {
			w.UpdateNode(upd.node, upd.update.Message, upd.update.Status)
			w.PrintProgress(rep)
		}
	}()

	for _, k8sNodeName := range nodeNames {
		eg.Go(func() error {
			reportFn := func(upd reporter.Update) {
				updateCh <- nodeUpdate{node: k8sNodeName, update: upd}
			}

			if !afterFailedDrain {
				reportFn(reporter.Update{
					Message: fmt.Sprintf("%s: waiting for node to become Ready", k8sNodeName),
					Status:  reporter.StatusRunning,
				})

				if waitErr := nodedrain.WaitForNodeReady(ctx, clientset, k8sNodeName, timeout); waitErr != nil {
					return fmt.Errorf("error waiting for node %q to become Ready: %w", k8sNodeName, waitErr)
				}
			}

			return nodedrain.Uncordon(ctx, clientset, k8sNodeName, reportFn)
		})
	}

	err = eg.Wait()

	close(updateCh)

	<-aggregatorDone

	return err
}
