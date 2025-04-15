// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/jsimonetti/rtnetlink/v2"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// AddressStatusController manages secrets.Etcd based on configuration.
type AddressStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *AddressStatusController) Name() string {
	return "network.AddressStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AddressStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *AddressStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.AddressStatusType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *AddressStatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	watcher, err := watch.NewRtNetlink(watch.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_LINK|unix.RTMGRP_IPV4_IFADDR|unix.RTMGRP_IPV6_IFADDR)
	if err != nil {
		return err
	}

	defer watcher.Done()

	conn, err := rtnetlink.Dial(nil)
	if err != nil {
		return fmt.Errorf("error dialing rtnetlink socket: %w", err)
	}

	defer conn.Close() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// build links lookup table
		links, err := conn.Link.List()
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		linkLookup := make(map[uint32]string, len(links))

		for _, link := range links {
			linkLookup[link.Index] = link.Attributes.Name
		}

		touchedIDs := map[resource.ID]struct{}{}

		addrs, err := conn.Address.List()
		if err != nil {
			return fmt.Errorf("error listing addresses: %w", err)
		}

		for _, addr := range addrs {
			// TODO: should we use local address actually?
			// from if_addr.h:
			// IFA_ADDRESS is prefix address, rather than local interface address.
			// * It makes no difference for normally configured broadcast interfaces,
			// * but for point-to-point IFA_ADDRESS is DESTINATION address,
			// * local address is supplied in IFA_LOCAL attribute.
			ipAddr, _ := netip.AddrFromSlice(addr.Attributes.Address)
			ipPrefix := netip.PrefixFrom(ipAddr, int(addr.PrefixLength))
			id := network.AddressID(linkLookup[addr.Index], ipPrefix)

			if err = safe.WriterModify(ctx, r, network.NewAddressStatus(network.NamespaceName, id), func(r *network.AddressStatus) error {
				status := r.TypedSpec()

				status.Address = ipPrefix
				status.Local, _ = netip.AddrFromSlice(addr.Attributes.Local)
				status.Broadcast, _ = netip.AddrFromSlice(addr.Attributes.Broadcast)
				status.Anycast, _ = netip.AddrFromSlice(addr.Attributes.Anycast)
				status.Multicast, _ = netip.AddrFromSlice(addr.Attributes.Multicast)
				status.LinkIndex = addr.Index
				status.LinkName = linkLookup[addr.Index]
				status.Family = nethelpers.Family(addr.Family)
				status.Scope = nethelpers.Scope(addr.Scope)
				status.Flags = nethelpers.AddressFlags(addr.Attributes.Flags)
				status.Priority = addr.Attributes.Priority

				return nil
			}); err != nil {
				return fmt.Errorf("error modifying resource: %w", err)
			}

			touchedIDs[id] = struct{}{}
		}

		// list resources for cleanup
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.AddressStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; ok {
				continue
			}

			if err = r.Destroy(ctx, res.Metadata()); err != nil {
				return fmt.Errorf("error deleting address status %s: %w", res, err)
			}
		}

		r.ResetRestartBackoff()
	}
}
