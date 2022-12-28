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
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/slices"
	"github.com/siderolabs/go-pointer"
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
			ID:        pointer.To(config.V1Alpha1ID),
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
func (ctrl *StaticEndpointController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		machineConfig, err := safe.ReaderGet[*config.MachineConfig](ctx, r, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting machine config: %w", err)
		}

		cpHostname := machineConfig.Config().Cluster().Endpoint().Hostname()

		var resolver net.Resolver

		addrs, err := resolver.LookupNetIP(ctx, "ip", cpHostname)
		if err != nil {
			return fmt.Errorf("error resolving %q: %w", cpHostname, err)
		}

		addrs = slices.Map(addrs, netip.Addr.Unmap)

		if err = safe.WriterModify(ctx, r, k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, k8s.ControlPlaneKubernetesEndpointsID), func(endpoint *k8s.Endpoint) error {
			endpoint.TypedSpec().Addresses = addrs

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying endpoint: %w", err)
		}
	}
}
