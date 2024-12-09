// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/value"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/addressutil"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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
		{
			Namespace: network.NamespaceName,
			Type:      network.NodeAddressSortAlgorithmType,
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
func (ctrl *NodeAddressController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	var addressStatusController AddressStatusController

	addressStatusControllerName := addressStatusController.Name()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// get algorithm to use
		algoRes, err := safe.ReaderGetByID[*network.NodeAddressSortAlgorithm](ctx, r, network.NodeAddressSortAlgorithmID)
		if err != nil {
			if state.IsNotFoundError(err) {
				// wait for the resource to appear
				continue
			}

			return fmt.Errorf("error getting sort algorithm: %w", err)
		}

		algo := algoRes.TypedSpec().Algorithm

		// fetch link and address status resources
		links, err := safe.ReaderListAll[*network.LinkStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		// build "link up" lookup table
		linksUp := make(map[uint32]struct{})

		for link := range links.All() {
			if link.TypedSpec().OperationalState == nethelpers.OperStateUp || link.TypedSpec().OperationalState == nethelpers.OperStateUnknown {
				// skip physical interfaces without carrier
				if !link.TypedSpec().Physical() || link.TypedSpec().LinkState {
					linksUp[link.TypedSpec().Index] = struct{}{}
				}
			}
		}

		// fetch list of filters
		filters, err := safe.ReaderListAll[*network.NodeAddressFilter](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing address filters: %w", err)
		}

		addressesList, err := safe.ReaderListAll[*network.AddressStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing links: %w", err)
		}

		addresses := safe.ToSlice(addressesList, func(a *network.AddressStatus) *network.AddressStatus { return a })

		compareFunc := addressutil.CompareByAlgorithm(algo)

		// filter out addresses which should be ignored
		addresses = xslices.FilterInPlace(addresses, func(addr *network.AddressStatus) bool {
			if addr.TypedSpec().Scope >= nethelpers.ScopeLink {
				return false
			}

			ip := addr.TypedSpec().Address

			if ip.Addr().IsLoopback() || ip.Addr().IsMulticast() || ip.Addr().IsLinkLocalUnicast() {
				return false
			}

			return true
		})

		slices.SortFunc(addresses, addressutil.CompareAddressStatuses(compareFunc))

		var (
			defaultAddress netip.Prefix
			current        []netip.Prefix
			routed         []netip.Prefix
			accumulative   []netip.Prefix
		)

		for _, addr := range addresses {
			ip := addr.TypedSpec().Address

			// set defaultAddress to the smallest IP from the alphabetically first link
			if addr.Metadata().Owner() == addressStatusControllerName {
				if value.IsZero(defaultAddress) {
					defaultAddress = ip
				}
			}

			// assume addresses from external IPs to be always up
			if _, up := linksUp[addr.TypedSpec().LinkIndex]; up || addr.TypedSpec().LinkName == externalLink {
				current = append(current, ip)
			}

			// routed: filter out external addresses and addresses from SideroLink
			if _, up := linksUp[addr.TypedSpec().LinkIndex]; up && addr.TypedSpec().LinkName != externalLink {
				if network.NotSideroLinkIP(ip.Addr()) {
					routed = append(routed, ip)
				}
			}

			accumulative = append(accumulative, ip)
		}

		// sort current addresses
		slices.SortFunc(current, compareFunc)
		slices.SortFunc(routed, compareFunc)

		// remove duplicates from current addresses
		current = addressutil.DeduplicateIPPrefixes(current)
		routed = addressutil.DeduplicateIPPrefixes(routed)

		touchedIDs := make(map[resource.ID]struct{})

		// update output resources
		if !value.IsZero(defaultAddress) {
			if err = safe.WriterModify(ctx, r, network.NewNodeAddress(network.NamespaceName, network.NodeAddressDefaultID), func(r *network.NodeAddress) error {
				spec := r.TypedSpec()

				// never overwrite default address if it's already set
				// we should start handing default address updates, but for now we're not ready
				//
				// at the same time check that recorded default address is still on the host, if it's not => replace it
				// also replace default address on algorithm change
				if spec.SortAlgorithm == algo && len(spec.Addresses) > 0 && slices.ContainsFunc(current, func(addr netip.Prefix) bool { return spec.Addresses[0] == addr }) {
					return nil
				}

				spec.Addresses = []netip.Prefix{defaultAddress}
				spec.SortAlgorithm = algo

				return nil
			}); err != nil {
				return fmt.Errorf("error updating output resource: %w", err)
			}

			touchedIDs[network.NodeAddressDefaultID] = struct{}{}
		}

		if err = ctrl.updateCurrentAddresses(ctx, r, network.NodeAddressCurrentID, current, algo); err != nil {
			return err
		}

		touchedIDs[network.NodeAddressCurrentID] = struct{}{}

		if err = ctrl.updateCurrentAddresses(ctx, r, network.NodeAddressRoutedID, routed, algo); err != nil {
			return err
		}

		touchedIDs[network.NodeAddressRoutedID] = struct{}{}

		if err = ctrl.updateAccumulativeAddresses(ctx, r, network.NodeAddressAccumulativeID, accumulative, algo); err != nil {
			return err
		}

		touchedIDs[network.NodeAddressAccumulativeID] = struct{}{}

		// update filtered resources
		for filterRes := range filters.All() {
			filterID := filterRes.Metadata().ID()
			filter := filterRes.TypedSpec()

			filteredCurrent := addressutil.FilterIPs(current, filter.IncludeSubnets, filter.ExcludeSubnets)
			filteredRouted := addressutil.FilterIPs(routed, filter.IncludeSubnets, filter.ExcludeSubnets)
			filteredAccumulative := addressutil.FilterIPs(accumulative, filter.IncludeSubnets, filter.ExcludeSubnets)

			if err = ctrl.updateCurrentAddresses(ctx, r, network.FilteredNodeAddressID(network.NodeAddressCurrentID, filterID), filteredCurrent, algo); err != nil {
				return err
			}

			if err = ctrl.updateCurrentAddresses(ctx, r, network.FilteredNodeAddressID(network.NodeAddressRoutedID, filterID), filteredRouted, algo); err != nil {
				return err
			}

			if err = ctrl.updateAccumulativeAddresses(ctx, r, network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, filterID), filteredAccumulative, algo); err != nil {
				return err
			}

			touchedIDs[network.FilteredNodeAddressID(network.NodeAddressCurrentID, filterID)] = struct{}{}
			touchedIDs[network.FilteredNodeAddressID(network.NodeAddressRoutedID, filterID)] = struct{}{}
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

		r.ResetRestartBackoff()
	}
}

func (ctrl *NodeAddressController) updateCurrentAddresses(ctx context.Context, r controller.Runtime, id resource.ID, current []netip.Prefix, algo nethelpers.AddressSortAlgorithm) error {
	if err := safe.WriterModify(ctx, r, network.NewNodeAddress(network.NamespaceName, id), func(r *network.NodeAddress) error {
		spec := r.TypedSpec()

		spec.Addresses = current
		spec.SortAlgorithm = algo

		return nil
	}); err != nil {
		return fmt.Errorf("error updating output resource: %w", err)
	}

	return nil
}

func (ctrl *NodeAddressController) updateAccumulativeAddresses(ctx context.Context, r controller.Runtime, id resource.ID, accumulative []netip.Prefix, algo nethelpers.AddressSortAlgorithm) error {
	if err := safe.WriterModify(ctx, r, network.NewNodeAddress(network.NamespaceName, id), func(r *network.NodeAddress) error {
		spec := r.TypedSpec()

		for _, ip := range accumulative {
			// find insert position using binary search
			pos, _ := slices.BinarySearchFunc(spec.Addresses, ip.Addr(), func(prefix netip.Prefix, addr netip.Addr) int {
				return prefix.Addr().Compare(ip.Addr())
			})

			if pos < len(spec.Addresses) && spec.Addresses[pos].Addr().Compare(ip.Addr()) == 0 {
				continue
			}

			// insert at position i
			spec.Addresses = slices.Insert(spec.Addresses, pos, ip)
		}

		spec.SortAlgorithm = algo

		return nil
	}); err != nil {
		return fmt.Errorf("error updating output resource: %w", err)
	}

	return nil
}
