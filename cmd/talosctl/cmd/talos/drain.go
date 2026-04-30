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
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/kubeclient"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/nodedrain"
	"github.com/siderolabs/talos/pkg/machinery/client"
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
// It returns a map of talosIP -> k8sNodeName for use in the uncordon phase.
// The context must NOT have "nodes" metadata set (use WithClientAndNodes).
func drainNodes(ctx context.Context, c *client.Client, nodes []string, drainTimeout time.Duration, rep *reporter.Reporter) (map[string]string, error) {
	// Fetch kubeconfig once - it is cluster-global, not node-specific.
	clientset, err := kubeclient.FromTalosClient(ctx, c)
	if err != nil {
		return nil, fmt.Errorf("error creating Kubernetes client for drain: %w", err)
	}

	// Channel for per-node progress updates -> single aggregator goroutine.
	updateCh := make(chan nodeUpdate)

	// k8sNames collects Talos IP -> K8s node name mappings produced by each goroutine.
	k8sNames := make(map[string]string, len(nodes))

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
	for _, node := range nodes {
		eg.Go(func() error {
			// Use client.WithNode for single-node COSI routing (avoids one-to-many proxy error).
			nodeCtx := client.WithNode(ctx, node)

			k8sNodeName, resolveErr := nodedrain.GetKubernetesNodeName(nodeCtx, c)
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
		return nil, err
	}

	return k8sNames, nil
}

// uncordonNodes runs Phase 3: waits for each Kubernetes node to become Ready,
// then uncordons all of them in parallel.
//
// nodeNames maps talosIP -> k8sNodeName (produced by drainNodes).
// The context must NOT have "nodes" metadata set (use WithClientAndNodes).
func uncordonNodes(ctx context.Context, c *client.Client, nodeNames map[string]string, timeout time.Duration, rep *reporter.Reporter) error {
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

			reportFn(reporter.Update{
				Message: fmt.Sprintf("%s: waiting for node to become Ready", k8sNodeName),
				Status:  reporter.StatusRunning,
			})

			if waitErr := nodedrain.WaitForNodeReady(ctx, clientset, k8sNodeName, timeout); waitErr != nil {
				return fmt.Errorf("error waiting for node %q to become Ready: %w", k8sNodeName, waitErr)
			}

			return nodedrain.Uncordon(ctx, clientset, k8sNodeName, reportFn)
		})
	}

	err = eg.Wait()

	close(updateCh)

	<-aggregatorDone

	return err
}
