// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/jsimonetti/rtnetlink"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// AddressSpecController applies network.AddressSpec to the actual interfaces.
type AddressSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *AddressSpecController) Name() string {
	return "network.AddressSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AddressSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.AddressSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *AddressSpecController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,dupl
func (ctrl *AddressSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// watch link changes as some address might need to be re-applied if the link appears
	watcher, err := watch.NewRtNetlink(r, unix.RTMGRP_LINK)
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

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.AddressSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// add finalizers for all live resources
		for _, res := range list.Items {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		// list rtnetlink links (interfaces)
		links, err := conn.Link.List()
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		// list rtnetlink addresses
		addrs, err := conn.Address.List()
		if err != nil {
			return fmt.Errorf("error listing addresses: %w", err)
		}

		// loop over addresses and make reconcile decision
		for _, res := range list.Items {
			address := res.(*network.AddressSpec) //nolint:forcetypeassert,errcheck

			if err = ctrl.syncAddress(ctx, r, logger, conn, links, addrs, address); err != nil {
				return err
			}
		}
	}
}

func resolveLinkName(links []rtnetlink.LinkMessage, linkName string) uint32 {
	for _, link := range links {
		if link.Attributes.Name == linkName {
			return link.Index
		}
	}

	return 0
}

func findAddress(addrs []rtnetlink.AddressMessage, linkIndex uint32, ipPrefix netaddr.IPPrefix) *rtnetlink.AddressMessage {
	for i, addr := range addrs {
		if addr.Index != linkIndex {
			continue
		}

		if addr.PrefixLength != ipPrefix.Bits {
			continue
		}

		if !addr.Attributes.Address.Equal(ipPrefix.IP.IPAddr().IP) {
			continue
		}

		return &addrs[i]
	}

	return nil
}

//nolint:gocyclo
func (ctrl *AddressSpecController) syncAddress(ctx context.Context, r controller.Runtime, logger *zap.Logger, conn *rtnetlink.Conn,
	links []rtnetlink.LinkMessage, addrs []rtnetlink.AddressMessage, address *network.AddressSpec) error {
	linkIndex := resolveLinkName(links, address.Status().LinkName)

	switch address.Metadata().Phase() {
	case resource.PhaseTearingDown:
		if linkIndex == 0 {
			// address should be deleted, but link is gone, so assume address is gone
			if err := r.RemoveFinalizer(ctx, address.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error removing finalizer: %w", err)
			}

			return nil
		}

		if existing := findAddress(addrs, linkIndex, address.Status().Address); existing != nil {
			// delete address
			if err := conn.Address.Delete(existing); err != nil {
				return fmt.Errorf("error removing address: %w", err)
			}

			logger.Sugar().Infof("removed address %s from %q", address.Status().Address, address.Status().LinkName)
		}

		// now remove finalizer as address was deleted
		if err := r.RemoveFinalizer(ctx, address.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer: %w", err)
		}
	case resource.PhaseRunning:
		if linkIndex == 0 {
			// address can't be assigned as link doesn't exist (yet), skip it
			return nil
		}

		if existing := findAddress(addrs, linkIndex, address.Status().Address); existing != nil {
			// check if existing matches the spec: if it does, skip update
			if existing.Scope == uint8(address.Status().Scope) && existing.Flags == uint8(address.Status().Flags) &&
				existing.Attributes.Flags == uint32(address.Status().Flags) {
				return nil
			}

			// delete address to get new one assigned below
			if err := conn.Address.Delete(existing); err != nil {
				return fmt.Errorf("error removing address: %w", err)
			}

			logger.Sugar().Infof("removed address %s from %q", address.Status().Address, address.Status().LinkName)
		}

		// add address
		if err := conn.Address.New(&rtnetlink.AddressMessage{
			Family:       uint8(address.Status().Family),
			PrefixLength: address.Status().Address.Bits,
			Flags:        uint8(address.Status().Flags),
			Scope:        uint8(address.Status().Scope),
			Index:        linkIndex,
			Attributes: rtnetlink.AddressAttributes{
				Address:   address.Status().Address.IP.IPAddr().IP,
				Local:     address.Status().Address.IP.IPAddr().IP,
				Broadcast: broadcastAddr(address.Status().Address),
				Flags:     uint32(address.Status().Flags),
			},
		}); err != nil {
			return fmt.Errorf("error adding address: %w", err)
		}

		logger.Sugar().Infof("assigned address %s to %q", address.Status().Address, address.Status().LinkName)
	}

	return nil
}

func broadcastAddr(addr netaddr.IPPrefix) net.IP {
	if !addr.IP.Is4() {
		return nil
	}

	ipnet := addr.IPNet()

	ip := ipnet.IP.To4()
	if ip == nil {
		return nil
	}

	mask := net.IP(ipnet.Mask).To4()

	n := len(ip)
	if n != len(mask) {
		return nil
	}

	out := make(net.IP, n)

	for i := 0; i < n; i++ {
		out[i] = ip[i] | ^mask[i]
	}

	return out
}
