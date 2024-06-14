// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package resourcedata implements the types and the data sources for the data sourced from the Talos resource API (COSI).
package resourcedata

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/channel"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/pkg/dashboard/util"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// Data contains a resource, whether it is deleted and the node it came from.
type Data struct {
	Node     string
	Resource resource.Resource
	Deleted  bool
}

// Source is the data source for the Talos resources.
type Source struct {
	ctxCancel context.CancelFunc

	eg   errgroup.Group
	once sync.Once

	COSI state.State

	ch             chan Data
	NodeResourceCh <-chan Data
}

// Run starts the data source.
func (source *Source) Run(ctx context.Context) {
	source.once.Do(func() {
		source.run(ctx)
	})
}

// Stop stops the data source.
func (source *Source) Stop() error {
	source.ctxCancel()

	return source.eg.Wait()
}

func (source *Source) run(ctx context.Context) {
	ctx, source.ctxCancel = context.WithCancel(ctx)

	source.ch = make(chan Data)

	source.NodeResourceCh = source.ch

	for _, nodeContext := range util.NodeContexts(ctx) {
		source.eg.Go(func() error {
			source.runResourceWatchWithRetries(nodeContext.Ctx, nodeContext.Node)

			return nil
		})
	}
}

func (source *Source) runResourceWatchWithRetries(ctx context.Context, node string) {
	for {
		if err := source.runResourceWatch(ctx, node); errors.Is(err, context.Canceled) {
			return
		}

		// wait for a second before the next retry
		timer := time.NewTimer(1 * time.Second)

		select {
		case <-ctx.Done():
			timer.Stop()

			return
		case <-timer.C:
		}
	}
}

//nolint:gocyclo,cyclop
func (source *Source) runResourceWatch(ctx context.Context, node string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	eventCh := make(chan state.Event)

	if err := source.COSI.Watch(ctx, runtime.NewMachineStatus().Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, runtime.NewSecurityStateSpec(v1alpha1.NamespaceName).Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, config.NewMachineType().Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID).Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, network.NewResolverStatus(network.NamespaceName, network.ResolverID).Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, network.NewTimeServerStatus(network.NamespaceName, network.TimeServerID).Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, hardware.NewSystemInformation(hardware.SystemInformationID).Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, cluster.NewInfo().Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, network.NewStatus(network.NamespaceName, network.StatusID).Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.Watch(ctx, network.NewHostnameStatus(network.NamespaceName, network.HostnameID).Metadata(), eventCh); err != nil {
		return err
	}

	if err := source.COSI.WatchKind(ctx, runtime.NewMetaKey(runtime.NamespaceName, "").Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	if err := source.COSI.WatchKind(ctx, k8s.NewStaticPodStatus(k8s.NamespaceName, "").Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	if err := source.COSI.WatchKind(ctx, network.NewRouteStatus(network.NamespaceName, "").Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	if err := source.COSI.WatchKind(ctx, network.NewLinkStatus(network.NamespaceName, "").Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	if err := source.COSI.WatchKind(ctx, cluster.NewMember(cluster.NamespaceName, "").Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	if err := source.COSI.WatchKind(ctx, network.NewNodeAddress(network.NamespaceName, "").Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	if err := source.COSI.WatchKind(ctx, siderolink.NewStatus().Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		return err
	}

	if err := source.COSI.WatchKind(ctx, runtime.NewDiagnostic(runtime.NamespaceName, "").Metadata(), eventCh, state.WithBootstrapContents(true)); err != nil {
		if client.StatusCode(err) != codes.PermissionDenied {
			// ignore permission denied, means resource is not supported yet
			return err
		}
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event := <-eventCh:
			switch event.Type {
			case state.Errored:
				return fmt.Errorf("watch failed: %w", event.Error)
			case state.Bootstrapped:
				// ignored
			case state.Created, state.Updated:
				if !channel.SendWithContext(ctx, source.ch, Data{
					Node:     node,
					Resource: event.Resource,
				}) {
					return ctx.Err()
				}
			case state.Destroyed:
				if !channel.SendWithContext(ctx, source.ch, Data{
					Node:     node,
					Resource: event.Resource,
					Deleted:  true,
				}) {
					return ctx.Err()
				}
			}
		}
	}
}
