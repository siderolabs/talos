// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/jsimonetti/rtnetlink/v2"
	"github.com/mdlayher/arp"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"
	"go4.org/netipx"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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
//nolint:gocyclo
func (ctrl *AddressSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// watch link changes as some address might need to be re-applied if the link appears
	watcher, err := watch.NewRtNetlink(watch.NewDefaultRateLimitedTrigger(ctx, r), unix.RTMGRP_LINK)
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
		list, err := safe.ReaderList[*network.AddressSpec](ctx, r, resource.NewMetadata(network.NamespaceName, network.AddressSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// add finalizers for all live resources
		for res := range list.All() {
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
		for address := range list.All() {
			if err = ctrl.syncAddress(ctx, r, logger, conn, links, addrs, address); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

func resolveLinkName(links []rtnetlink.LinkMessage, linkName string) uint32 {
	if linkName == "" {
		return 0 // should never match
	}

	// first, lookup by name
	for _, link := range links {
		if link.Attributes.Name == linkName {
			return link.Index
		}
	}

	// then, lookup by alias/altname
	for _, link := range links {
		if pointer.SafeDeref(link.Attributes.Alias) == linkName {
			return link.Index
		}

		if slices.Index(link.Attributes.AltNames, linkName) != -1 {
			return link.Index
		}
	}

	return 0
}

func findAddress(addrs []rtnetlink.AddressMessage, linkIndex uint32, ipPrefix netip.Prefix) *rtnetlink.AddressMessage {
	for i, addr := range addrs {
		if addr.Index != linkIndex {
			continue
		}

		if int(addr.PrefixLength) != ipPrefix.Bits() {
			continue
		}

		if !addr.Attributes.Address.Equal(ipPrefix.Addr().AsSlice()) {
			continue
		}

		return &addrs[i]
	}

	return nil
}

//nolint:gocyclo
func (ctrl *AddressSpecController) syncAddress(ctx context.Context, r controller.Runtime, logger *zap.Logger, conn *rtnetlink.Conn,
	links []rtnetlink.LinkMessage, addrs []rtnetlink.AddressMessage, address *network.AddressSpec,
) error {
	linkIndex := resolveLinkName(links, address.TypedSpec().LinkName)

	switch address.Metadata().Phase() {
	case resource.PhaseTearingDown:
		if linkIndex == 0 {
			// address should be deleted, but link is gone, so assume address is gone
			if err := r.RemoveFinalizer(ctx, address.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error removing finalizer: %w", err)
			}

			return nil
		}

		if existing := findAddress(addrs, linkIndex, address.TypedSpec().Address); existing != nil {
			// delete address
			if err := conn.Address.Delete(existing); err != nil {
				return fmt.Errorf("error removing address: %w", err)
			}

			logger.Sugar().Infof("removed address %s from %q", address.TypedSpec().Address, address.TypedSpec().LinkName)
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

		if existing := findAddress(addrs, linkIndex, address.TypedSpec().Address); existing != nil {
			// clear out tentative flag, it is set by the kernel, we shouldn't try to enforce it
			existing.Flags &= ^uint8(nethelpers.AddressTentative)
			existing.Attributes.Flags &= ^uint32(nethelpers.AddressTentative)

			// check if existing matches the spec: if it does, skip update
			if existing.Scope == uint8(address.TypedSpec().Scope) && existing.Flags == uint8(address.TypedSpec().Flags) &&
				existing.Attributes.Flags == uint32(address.TypedSpec().Flags) && existing.Attributes.Priority == address.TypedSpec().Priority {
				return nil
			}

			logger.Debug("replacing address",
				zap.Stringer("address", address.TypedSpec().Address),
				zap.String("link", address.TypedSpec().LinkName),
				zap.Stringer("old_scope", nethelpers.Scope(existing.Scope)),
				zap.Stringer("new_scope", address.TypedSpec().Scope),
				zap.Stringer("old_flags", nethelpers.AddressFlags(existing.Attributes.Flags)),
				zap.Stringer("new_flags", address.TypedSpec().Flags),
				zap.Uint32("old_priority", existing.Attributes.Priority),
				zap.Uint32("new_priority", address.TypedSpec().Priority),
			)

			// delete address to get new one assigned below
			if err := conn.Address.Delete(existing); err != nil {
				return fmt.Errorf("error removing address: %w", err)
			}

			logger.Info("removed address", zap.Stringer("address", address.TypedSpec().Address), zap.String("link", address.TypedSpec().LinkName))
		}

		// add address
		if err := conn.Address.New(&rtnetlink.AddressMessage{
			Family:       uint8(address.TypedSpec().Family),
			PrefixLength: uint8(address.TypedSpec().Address.Bits()),
			Flags:        uint8(address.TypedSpec().Flags),
			Scope:        uint8(address.TypedSpec().Scope),
			Index:        linkIndex,
			Attributes: &rtnetlink.AddressAttributes{
				Address:   address.TypedSpec().Address.Addr().AsSlice(),
				Local:     address.TypedSpec().Address.Addr().AsSlice(),
				Broadcast: broadcastAddr(address.TypedSpec().Address),
				Flags:     uint32(address.TypedSpec().Flags),
				Priority:  address.TypedSpec().Priority,
			},
		}); err != nil {
			// ignore EEXIST error
			if !errors.Is(err, os.ErrExist) {
				return fmt.Errorf("error adding address %s to %q: %w", address.TypedSpec().Address, address.TypedSpec().LinkName, err)
			}
		}

		logger.Info("assigned address", zap.Stringer("address", address.TypedSpec().Address), zap.String("link", address.TypedSpec().LinkName))

		if address.TypedSpec().AnnounceWithARP {
			if err := ctrl.gratuitousARP(logger, linkIndex, address.TypedSpec().Address.Addr()); err != nil {
				logger.Warn("failure sending gratuitous ARP", zap.Stringer("address", address.TypedSpec().Address), zap.String("link", address.TypedSpec().LinkName), zap.Error(err))
			}
		}
	}

	return nil
}

func (ctrl *AddressSpecController) gratuitousARP(logger *zap.Logger, linkIndex uint32, ip netip.Addr) error {
	etherBroadcast := net.HardwareAddr{0xff, 0xff, 0xff, 0xff, 0xff, 0xff}

	if !ip.Is4() {
		return nil
	}

	iface, err := net.InterfaceByIndex(int(linkIndex))
	if err != nil {
		return err
	}

	if len(iface.HardwareAddr) != 6 {
		// not ethernet
		return nil
	}

	cli, err := arp.Dial(iface)
	if err != nil {
		return fmt.Errorf("error creating arp client: %w", err)
	}

	defer cli.Close() //nolint:errcheck

	packet, err := arp.NewPacket(arp.OperationRequest, cli.HardwareAddr(), ip, cli.HardwareAddr(), ip)
	if err != nil {
		return fmt.Errorf("error building packet: %w", err)
	}

	if err = cli.WriteTo(packet, etherBroadcast); err != nil {
		return fmt.Errorf("error sending gratuitous ARP: %w", err)
	}

	logger.Info("sent gratuitous ARP", zap.Stringer("address", ip), zap.String("link", iface.Name))

	return nil
}

func broadcastAddr(addr netip.Prefix) net.IP {
	if !addr.Addr().Is4() {
		return nil
	}

	ipnet := netipx.PrefixIPNet(addr)

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

	for i := range n {
		out[i] = ip[i] | ^mask[i]
	}

	return out
}
