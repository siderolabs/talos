// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"sort"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

// NodeAddressController manages secrets.Etcd based on configuration.
type NodeAddressController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodeAddressController) Name() string {
	return "network.NodeAddressController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeAddressController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.AddressStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.LinkStatusType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressFilterType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeAddressController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.NodeAddressType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *NodeAddressController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	var addressStatusController AddressStatusController

	addressStatusControllerName := addressStatusController.Name()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// fetch link and address status resources
		links, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.LinkStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		// build "link up" lookup table
		linksUp := make(map[uint32]struct{})

		for _, r := range links.Items {
			link := r.(*network.LinkStatus) //nolint:errcheck,forcetypeassert

			if link.TypedSpec().OperationalState == nethelpers.OperStateUp || link.TypedSpec().OperationalState == nethelpers.OperStateUnknown {
				// skip physical interfaces without carrier
				if !link.Physical() || link.TypedSpec().LinkState {
					linksUp[link.TypedSpec().Index] = struct{}{}
				}
			}
		}

		// fetch list of filters
		filters, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressFilterType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing address filters: %w", err)
		}

		addresses, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.AddressStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		var (
			defaultAddress      netaddr.IPPrefix
			defaultAddrLinkName string
			current             []netaddr.IPPrefix
			accumulative        []netaddr.IPPrefix
		)

		for _, r := range addresses.Items {
			addr := r.(*network.AddressStatus) //nolint:errcheck,forcetypeassert

			if addr.TypedSpec().Scope >= nethelpers.ScopeLink {
				continue
			}

			ip := addr.TypedSpec().Address

			if ip.IP().IsLoopback() || ip.IP().IsMulticast() || ip.IP().IsLinkLocalUnicast() {
				continue
			}

			if network.IsULA(ip.IP(), network.ULASideroLink) {
				// ignore SideroLink addresses, as they are point-to-point addresses
				continue
			}

			// set defaultAddress to the smallest IP from the alphabetically first link
			// ignore address which are not assigned from the physical links
			if addr.Metadata().Owner() == addressStatusControllerName {
				if defaultAddress.IsZero() || addr.TypedSpec().LinkName < defaultAddrLinkName || (addr.TypedSpec().LinkName == defaultAddrLinkName && ip.IP().Compare(defaultAddress.IP()) < 0) {
					defaultAddress = ip
					defaultAddrLinkName = addr.TypedSpec().LinkName
				}
			}

			// assume addresses from external IPs to be always up
			if _, up := linksUp[addr.TypedSpec().LinkIndex]; up || addr.TypedSpec().LinkName == externalLink {
				current = append(current, ip)
			}

			accumulative = append(accumulative, ip)
		}

		// sort current addresses
		sort.Slice(current, func(i, j int) bool { return current[i].IP().Compare(current[j].IP()) < 0 })

		// remove duplicates from current addresses
		current = deduplicateIPPrefixes(current)

		touchedIDs := make(map[resource.ID]struct{})

		// update output resources
		if !defaultAddress.IsZero() {
			if err = r.Modify(ctx, network.NewNodeAddress(network.NamespaceName, network.NodeAddressDefaultID), func(r resource.Resource) error {
				spec := r.(*network.NodeAddress).TypedSpec()

				// never overwrite default address if it's already set
				// we should start handing default address updates, but for now we're not ready
				if len(spec.Addresses) > 0 {
					return nil
				}

				spec.Addresses = []netaddr.IPPrefix{defaultAddress}

				return nil
			}); err != nil {
				return fmt.Errorf("error updating output resource: %w", err)
			}

			touchedIDs[network.NodeAddressDefaultID] = struct{}{}
		}

		if err = updateCurrentAddresses(ctx, r, network.NodeAddressCurrentID, current); err != nil {
			return err
		}

		touchedIDs[network.NodeAddressCurrentID] = struct{}{}

		if err = updateAccumulativeAddresses(ctx, r, network.NodeAddressAccumulativeID, accumulative); err != nil {
			return err
		}

		touchedIDs[network.NodeAddressAccumulativeID] = struct{}{}

		// update filtered resources
		for _, res := range filters.Items {
			filterID := res.Metadata().ID()
			filter := res.(*network.NodeAddressFilter).TypedSpec()

			filteredCurrent := filterIPs(current, filter.IncludeSubnets, filter.ExcludeSubnets)
			filteredAccumulative := filterIPs(accumulative, filter.IncludeSubnets, filter.ExcludeSubnets)

			if err = updateCurrentAddresses(ctx, r, network.FilteredNodeAddressID(network.NodeAddressCurrentID, filterID), filteredCurrent); err != nil {
				return err
			}

			if err = updateAccumulativeAddresses(ctx, r, network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filterID), filteredAccumulative); err != nil {
				return err
			}

			touchedIDs[network.FilteredNodeAddressID(network.NodeAddressCurrentID, filterID)] = struct{}{}
			touchedIDs[network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filterID)] = struct{}{}
		}

		// list keys for cleanup
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.NodeAddressType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}
	}
}

func deduplicateIPPrefixes(current []netaddr.IPPrefix) []netaddr.IPPrefix {
	// assumes that current is sorted
	n := 0

	var prev netaddr.IPPrefix

	for _, x := range current {
		if prev != x {
			current[n] = x
			n++
		}

		prev = x
	}

	return current[:n]
}

func filterIPs(addrs []netaddr.IPPrefix, includeSubnets, excludeSubnets []netaddr.IPPrefix) []netaddr.IPPrefix {
	result := make([]netaddr.IPPrefix, 0, len(addrs))

outer:
	for _, ip := range addrs {
		if len(includeSubnets) > 0 {
			matchesAny := false

			for _, subnet := range includeSubnets {
				if subnet.Overlaps(ip) {
					matchesAny = true

					break
				}
			}

			if !matchesAny {
				continue outer
			}
		}

		for _, subnet := range excludeSubnets {
			if subnet.Overlaps(ip) {
				continue outer
			}
		}

		result = append(result, ip)
	}

	return result
}

func updateCurrentAddresses(ctx context.Context, r controller.Runtime, id resource.ID, current []netaddr.IPPrefix) error {
	if err := r.Modify(ctx, network.NewNodeAddress(network.NamespaceName, id), func(r resource.Resource) error {
		spec := r.(*network.NodeAddress).TypedSpec()

		spec.Addresses = current

		return nil
	}); err != nil {
		return fmt.Errorf("error updating output resource: %w", err)
	}

	return nil
}

func updateAccumulativeAddresses(ctx context.Context, r controller.Runtime, id resource.ID, accumulative []netaddr.IPPrefix) error {
	if err := r.Modify(ctx, network.NewNodeAddress(network.NamespaceName, id), func(r resource.Resource) error {
		spec := r.(*network.NodeAddress).TypedSpec()

		for _, ip := range accumulative {
			ip := ip

			// find insert position using binary search
			i := sort.Search(len(spec.Addresses), func(j int) bool {
				return !spec.Addresses[j].IP().Less(ip.IP())
			})

			if i < len(spec.Addresses) && spec.Addresses[i].IP().Compare(ip.IP()) == 0 {
				continue
			}

			// insert at position i
			spec.Addresses = append(spec.Addresses, netaddr.IPPrefix{})
			copy(spec.Addresses[i+1:], spec.Addresses[i:])
			spec.Addresses[i] = ip
		}

		return nil
	}); err != nil {
		return fmt.Errorf("error updating output resource: %w", err)
	}

	return nil
}
