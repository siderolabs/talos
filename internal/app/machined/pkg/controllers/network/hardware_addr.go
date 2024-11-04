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
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// HardwareAddrController manages secrets.Etcd based on configuration.
type HardwareAddrController struct{}

// Name implements controller.Controller interface.
func (ctrl *HardwareAddrController) Name() string {
	return "network.HardwareAddrController"
}

// Inputs implements controller.Controller interface.
func (ctrl *HardwareAddrController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *HardwareAddrController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.HardwareAddrType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *HardwareAddrController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list the existing HardwareAddr resources and mark them all to be deleted, as the actual link is discovered via netlink, resource ID is removed from the list
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.HardwareAddrType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		itemsToDelete := map[resource.ID]struct{}{}

		for _, r := range list.Items {
			itemsToDelete[r.Metadata().ID()] = struct{}{}
		}

		// list links and find the first physical link
		links, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range links.Items {
			link := res.(*network.LinkStatus) //nolint:errcheck,forcetypeassert

			if !link.TypedSpec().Physical() {
				continue
			}

			if err = safe.WriterModify(ctx, r, network.NewHardwareAddr(network.NamespaceName, network.FirstHardwareAddr), func(r *network.HardwareAddr) error {
				spec := r.TypedSpec()

				spec.HardwareAddr = link.TypedSpec().HardwareAddr
				spec.Name = link.Metadata().ID()

				return nil
			}); err != nil {
				return fmt.Errorf("error modifying resource: %w", err)
			}

			delete(itemsToDelete, network.FirstHardwareAddr)

			// as link status are listed in sorted order, first physical link in the list is the one we need
			break
		}

		for id := range itemsToDelete {
			if err = r.Destroy(ctx, resource.NewMetadata(network.NamespaceName, network.HardwareAddrType, id, resource.VersionUndefined)); err != nil {
				return fmt.Errorf("error deleting resource %q: %w", id, err)
			}
		}

		r.ResetRestartBackoff()
	}
}
