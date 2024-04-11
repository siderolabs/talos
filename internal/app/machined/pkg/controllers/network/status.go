// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/value"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// StatusController manages network.Status based on state of other resources.
type StatusController struct {
	V1Alpha1Mode runtime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *StatusController) Name() string {
	return "network.StatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressType,
			ID:        optional.Some(network.NodeAddressCurrentID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.RouteStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: files.NamespaceName,
			Type:      files.EtcFileStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.ProbeStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.StatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *StatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		result := network.StatusSpec{}

		// addresses
		currentAddresses, err := safe.ReaderGet[*network.NodeAddress](ctx, r, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.NodeAddressCurrentID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting resource: %w", err)
			}
		} else {
			result.AddressReady = len(currentAddresses.TypedSpec().Addresses) > 0
		}

		// connectivity
		// if any probes are defined, use their status, otherwise rely on presence of the default gateway
		probeStatuses, err := safe.ReaderListAll[*network.ProbeStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error getting probe statuses: %w", err)
		}

		allProbesSuccess := true

		for iter := probeStatuses.Iterator(); iter.Next(); {
			if !iter.Value().TypedSpec().Success {
				allProbesSuccess = false

				break
			}
		}

		if probeStatuses.Len() > 0 && allProbesSuccess {
			result.ConnectivityReady = true
		} else if probeStatuses.Len() == 0 {
			var routes safe.List[*network.RouteStatus]

			routes, err = safe.ReaderListAll[*network.RouteStatus](ctx, r)
			if err != nil {
				return fmt.Errorf("error getting routes: %w", err)
			}

			for iter := routes.Iterator(); iter.Next(); {
				if value.IsZero(iter.Value().TypedSpec().Destination) {
					result.ConnectivityReady = true

					break
				}
			}
		}

		// hostname
		_, err = r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting resource: %w", err)
			}
		} else {
			result.HostnameReady = true
		}

		// etc files
		result.EtcFilesReady = true

		for _, requiredFile := range []string{"hosts", "resolv.conf"} {
			// in container mode, ignore resolv.conf, it's managed by the container runtime
			if ctrl.V1Alpha1Mode.InContainer() && requiredFile == "resolv.conf" {
				continue
			}

			_, err = r.Get(ctx, resource.NewMetadata(files.NamespaceName, files.EtcFileStatusType, requiredFile, resource.VersionUndefined))
			if err != nil {
				if !state.IsNotFoundError(err) {
					return fmt.Errorf("error getting resource: %w", err)
				}

				result.EtcFilesReady = false

				break
			}
		}

		// update output status
		if err = safe.WriterModify(ctx, r, network.NewStatus(network.NamespaceName, network.StatusID),
			func(r *network.Status) error {
				*r.TypedSpec() = result

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying output status: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
