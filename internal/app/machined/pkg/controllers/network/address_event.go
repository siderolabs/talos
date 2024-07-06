// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// AddressEventController reports aggregated enpoints state from hostname statuses and k8s endpoints
// to the events stream.
type AddressEventController struct {
	V1Alpha1Events runtime.Publisher
}

// Name implements controller.Controller interface.
func (ctrl *AddressEventController) Name() string {
	return "network.AddressEventController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AddressEventController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			Kind:      controller.InputWeak,
			ID: optional.Some(network.FilteredNodeAddressID(
				network.NodeAddressCurrentID,
				k8s.NodeAddressFilterNoK8s)),
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			Kind:      controller.InputWeak,
			ID:        optional.Some(network.HostnameID),
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *AddressEventController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *AddressEventController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ticker := time.NewTicker(time.Minute * 10)

	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		case <-r.EventCh():
		}

		var addresses []string

		nodeAddr, err := r.Get(
			ctx,
			resource.NewMetadata(
				network.NamespaceName,
				network.NodeAddressType,
				network.FilteredNodeAddressID(
					network.NodeAddressCurrentID,
					k8s.NodeAddressFilterNoK8s),
				resource.VersionUndefined),
		)
		if err != nil {
			if !state.IsNotFoundError(err) {
				return err
			}
		} else {
			for _, addr := range nodeAddr.(*network.NodeAddress).TypedSpec().Addresses {
				addresses = append(
					addresses,
					addr.Addr().String(),
				)
			}
		}

		var hostname string

		hostnameStatus, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return err
			}
		} else {
			hostname = hostnameStatus.(*network.HostnameStatus).TypedSpec().Hostname
		}

		ctrl.V1Alpha1Events.Publish(ctx, &machine.AddressEvent{
			Hostname:  hostname,
			Addresses: addresses,
		})

		r.ResetRestartBackoff()
	}
}
