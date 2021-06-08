// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/talos-systems/talos/pkg/resources/files"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// StatusController manages secrets.Etcd based on configuration.
type StatusController struct{}

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
			ID:        pointer.ToString(network.NodeAddressCurrentID),
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
//nolint:gocyclo
func (ctrl *StatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		result := network.StatusSpec{}

		// addresses
		currentAddresses, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.NodeAddressCurrentID, resource.VersionUndefined))
		if err != nil {
			if !state.IsNotFoundError(err) {
				return fmt.Errorf("error getting resource: %w", err)
			}
		} else {
			result.AddressReady = len(currentAddresses.(*network.NodeAddress).TypedSpec().Addresses) > 0
		}

		// connectivity
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.RouteStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error getting routes: %w", err)
		}

		for _, item := range list.Items {
			if item.(*network.RouteStatus).TypedSpec().Destination.IsZero() {
				result.ConnectivityReady = true

				break
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
		if err = r.Modify(ctx, network.NewStatus(network.NamespaceName, network.StatusID),
			func(r resource.Resource) error {
				*r.(*network.Status).TypedSpec() = result

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying output status: %w", err)
		}
	}
}
