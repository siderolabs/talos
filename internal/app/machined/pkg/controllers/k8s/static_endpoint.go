// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// StaticEndpointController injects endpoints based on machine configuration.
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
			ID:        optional.Some(config.V1Alpha1ID),
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

		machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		r.StartTrackingOutputs()

		if machineConfig != nil && machineConfig.Config().Cluster() != nil {
			cpHostname := machineConfig.Config().Cluster().Endpoint().Hostname()

			var (
				resolver net.Resolver
				addrs    []netip.Addr
			)

			addrs, err = resolver.LookupNetIP(ctx, "ip", cpHostname)
			if err != nil {
				return fmt.Errorf("error resolving %q: %w", cpHostname, err)
			}

			addrs = xslices.Map(addrs, netip.Addr.Unmap)

			if err = safe.WriterModify(ctx, r, k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, k8s.ControlPlaneKubernetesEndpointsID), func(endpoint *k8s.Endpoint) error {
				endpoint.TypedSpec().Addresses = addrs

				return nil
			}); err != nil {
				return fmt.Errorf("error modifying endpoint: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*k8s.Endpoint](ctx, r); err != nil {
			return err
		}
	}
}
