// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

// MaintenanceConfigController manages Maintenance Service config: which address it should listen on, etc.
type MaintenanceConfigController struct{}

// Name implements controller.Controller interface.
func (ctrl *MaintenanceConfigController) Name() string {
	return "runtime.MaintenanceConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MaintenanceConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      siderolink.ConfigType,
			ID:        optional.Some(siderolink.ConfigID),
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.NodeAddressCurrentID),
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MaintenanceConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.MaintenanceServiceConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *MaintenanceConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		nodeAddresses, err := safe.ReaderGetByID[*network.NodeAddress](ctx, r, network.NodeAddressCurrentID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting node address: %w", err)
		}

		var (
			listenAddress      string
			reachableAddresses []netip.Addr
		)

		if nodeAddresses != nil {
			reachableAddresses = nodeAddresses.TypedSpec().IPs()
		}

		_, err = safe.ReaderGetByID[*siderolink.Config](ctx, r, siderolink.ConfigID)

		// check if SideroLink config exists:
		switch {
		// * if it exists, find the SideroLink address and listen only on it
		case err == nil:
			if nodeAddresses != nil {
				sideroLinkAddresses := xslices.Filter(nodeAddresses.TypedSpec().IPs(), func(addr netip.Addr) bool {
					return network.IsULA(addr, network.ULASideroLink)
				})

				if len(sideroLinkAddresses) > 0 {
					listenAddress = nethelpers.JoinHostPort(sideroLinkAddresses[0].String(), constants.ApidPort)
					reachableAddresses = sideroLinkAddresses[:1]
				}
			}
		// * if it doesn't exist, listen on '*'
		case state.IsNotFoundError(err):
			listenAddress = fmt.Sprintf(":%d", constants.ApidPort)
		default:
			return fmt.Errorf("error getting siderolink config: %w", err)
		}

		if listenAddress == "" {
			// drop config
			if err = r.Destroy(ctx, runtime.NewMaintenanceServiceConfig().Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error destroying maintenance config: %w", err)
			}
		} else {
			// create/update config
			if err = safe.WriterModify[*runtime.MaintenanceServiceConfig](ctx, r, runtime.NewMaintenanceServiceConfig(),
				func(config *runtime.MaintenanceServiceConfig) error {
					config.TypedSpec().ListenAddress = listenAddress
					config.TypedSpec().ReachableAddresses = reachableAddresses

					return nil
				}); err != nil {
				return fmt.Errorf("error updating maintenance config: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}
