// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	talosconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// StaticEndpointController injects endpoints based on machine configuration:
// static endpoints from the StaticEndpointsConfig document, and the cluster control plane endpoint.
type StaticEndpointController struct{}

// Name implements controller.Controller interface.
func (ctrl *StaticEndpointController) Name() string {
	return "k8s.StaticEndpointController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StaticEndpointController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StaticEndpointController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.EndpointType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *StaticEndpointController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		r.StartTrackingOutputs()

		// static endpoints go first, so that a failure to resolve the control plane endpoint doesn't block them
		if machineConfig != nil && machineConfig.Config().StaticEndpointsConfig() != nil {
			if err = ctrl.updateStaticEndpoints(ctx, r, machineConfig.Config().StaticEndpointsConfig()); err != nil {
				return err
			}
		}

		if machineConfig != nil && machineConfig.Config().K8sClusterConfig() != nil {
			if err = ctrl.updateControlPlaneEndpoint(ctx, r, machineConfig.Config().K8sClusterConfig().ClusterEndpoint().Hostname()); err != nil {
				return err
			}
		}

		if err = safe.CleanupOutputs[*k8s.Endpoint](ctx, r); err != nil {
			return err
		}
	}
}

func (ctrl *StaticEndpointController) updateStaticEndpoints(ctx context.Context, r controller.Runtime, staticEndpoints talosconfig.StaticEndpointsConfig) error {
	if err := safe.WriterModify(ctx, r, k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, k8s.ControlPlaneStaticEndpointsID), func(endpoint *k8s.Endpoint) error {
		endpoint.TypedSpec().Addresses = slices.Clone(staticEndpoints.Addresses())
		endpoint.TypedSpec().Hosts = slices.Clone(staticEndpoints.Hostnames())

		return nil
	}); err != nil {
		return fmt.Errorf("error modifying static endpoint: %w", err)
	}

	return nil
}

func (ctrl *StaticEndpointController) updateControlPlaneEndpoint(ctx context.Context, r controller.Runtime, cpHostname string) error {
	var (
		resolver net.Resolver
		hosts    []string
	)

	addrs, err := resolver.LookupNetIP(ctx, "ip", cpHostname)
	if err != nil {
		return fmt.Errorf("error resolving %q: %w", cpHostname, err)
	}

	addrs = xslices.Map(addrs, netip.Addr.Unmap)

	if len(addrs) != 1 || addrs[0].String() != cpHostname {
		hosts = []string{cpHostname}
	}

	if err = safe.WriterModify(ctx, r, k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, k8s.ControlPlaneKubernetesEndpointsID), func(endpoint *k8s.Endpoint) error {
		endpoint.TypedSpec().Addresses = addrs
		endpoint.TypedSpec().Hosts = hosts

		return nil
	}); err != nil {
		return fmt.Errorf("error modifying endpoint: %w", err)
	}

	return nil
}
