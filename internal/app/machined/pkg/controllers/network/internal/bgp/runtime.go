// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"fmt"
	"iter"
	"maps"
	"net/netip"
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ResolvedConfig is a BGP instance configuration resolved against current network runtime state.
type ResolvedConfig struct {
	Spec               network.BGPInstanceConfigSpec
	AdvertisedPrefixes []netip.Prefix
	RouterID           netip.Addr
}

// RuntimeState is an indexed snapshot of network link and address status.
type RuntimeState struct {
	linkNames        map[string]string
	linksByName      map[string]network.LinkStatusSpec
	linkNamesByIndex map[uint32]string
	addressLinks     map[netip.Addr][]string
	addressesByLink  map[uint32]map[netip.Addr]struct{}
	addresses        []network.AddressStatusSpec
}

// NewRuntimeStateFromResources builds an indexed network runtime snapshot from status resources.
func NewRuntimeStateFromResources(
	links iter.Seq[*network.LinkStatus],
	addresses iter.Seq[*network.AddressStatus],
) *RuntimeState {
	linkSpecs := map[string]network.LinkStatusSpec{}

	for status := range links {
		linkSpecs[status.Metadata().ID()] = *status.TypedSpec()
	}

	var addressSpecs []network.AddressStatusSpec

	for status := range addresses {
		addressSpecs = append(addressSpecs, *status.TypedSpec())
	}

	return NewRuntimeState(linkSpecs, addressSpecs)
}

// NewRuntimeState builds an indexed network runtime snapshot.
func NewRuntimeState(links map[string]network.LinkStatusSpec, addresses []network.AddressStatusSpec) *RuntimeState {
	state := &RuntimeState{
		linkNames:        map[string]string{},
		linksByName:      maps.Clone(links),
		linkNamesByIndex: map[uint32]string{},
		addressLinks:     map[netip.Addr][]string{},
		addressesByLink:  map[uint32]map[netip.Addr]struct{}{},
		addresses:        slices.Clone(addresses),
	}

	// Match network.LinkResolver's two-pass behavior: collect aliases first, then make canonical
	// names win over conflicting aliases. Sort names because the input is a map.
	linkNames := slices.Sorted(maps.Keys(links))

	for _, name := range linkNames {
		spec := links[name]

		if spec.Alias != "" {
			state.linkNames[spec.Alias] = name
		}

		for _, altName := range spec.AltNames {
			state.linkNames[altName] = name
		}
	}

	for _, name := range linkNames {
		spec := links[name]

		state.linkNames[name] = name
		state.linkNamesByIndex[spec.Index] = name
	}

	for _, spec := range addresses {
		address := spec.Address.Addr()
		linkName := state.resolveLinkName(spec.LinkName)

		state.addressLinks[address] = append(state.addressLinks[address], linkName)

		if state.addressesByLink[spec.LinkIndex] == nil {
			state.addressesByLink[spec.LinkIndex] = map[netip.Addr]struct{}{}
		}

		state.addressesByLink[spec.LinkIndex][address] = struct{}{}
	}

	return state
}

// Resolve resolves a BGP instance configuration and its derived runtime values.
func (state *RuntimeState) Resolve(spec network.BGPInstanceConfigSpec) (ResolvedConfig, error) {
	resolved := spec
	resolved.VRF = state.resolveLinkName(spec.VRF)
	resolved.AdvertiseLinks = slices.Clone(spec.AdvertiseLinks)
	resolved.Neighbors = slices.Clone(spec.Neighbors)

	if resolved.VRF != "" {
		status, exists := state.linksByName[resolved.VRF]
		if !exists {
			return ResolvedConfig{}, fmt.Errorf("VRF link %q is not ready", resolved.VRF)
		}

		if status.Kind != network.LinkKindVRF {
			return ResolvedConfig{}, fmt.Errorf("link %q is not a VRF", resolved.VRF)
		}
	}

	if resolved.RouteSource.IsValid() {
		if err := state.validateRouteSource(resolved.RouteSource, resolved.VRF); err != nil {
			return ResolvedConfig{}, fmt.Errorf("route source: %w", err)
		}
	}

	if err := state.resolveLinks(&resolved); err != nil {
		return ResolvedConfig{}, err
	}

	advertisedAddresses := state.advertisedAddresses(resolved.AdvertiseLinks)

	return ResolvedConfig{
		Spec:               resolved,
		AdvertisedPrefixes: connectedPrefixes(advertisedAddresses),
		RouterID:           routerID(resolved.RouterID, advertisedAddresses),
	}, nil
}

// LinkIndexByName returns a resolved link's kernel index.
func (state *RuntimeState) LinkIndexByName(name string) (uint32, bool) {
	spec, ok := state.linksByName[name]

	return spec.Index, ok
}

// OwnAddresses returns the addresses assigned to a link index.
func (state *RuntimeState) OwnAddresses(index uint32) map[netip.Addr]struct{} {
	return state.addressesByLink[index]
}

func (state *RuntimeState) resolveLinkName(name string) string {
	if resolved, ok := state.linkNames[name]; ok {
		return resolved
	}

	return name
}

func (state *RuntimeState) resolveLinks(resolved *network.BGPInstanceConfigSpec) error {
	for i, link := range resolved.AdvertiseLinks {
		resolved.AdvertiseLinks[i] = state.resolveLinkName(link)

		if err := state.validateLinkDomain(resolved.AdvertiseLinks[i], resolved.VRF); err != nil {
			return fmt.Errorf("advertised link: %w", err)
		}
	}

	for i := range resolved.Neighbors {
		if resolved.Neighbors[i].Link == "" {
			continue
		}

		resolved.Neighbors[i].Link = state.resolveLinkName(resolved.Neighbors[i].Link)

		if err := state.validateLinkDomain(resolved.Neighbors[i].Link, resolved.VRF); err != nil {
			return fmt.Errorf("neighbor link: %w", err)
		}
	}

	return nil
}

func (state *RuntimeState) validateRouteSource(source netip.Addr, vrf string) error {
	links := state.addressLinks[source]
	if len(links) == 0 {
		return fmt.Errorf("address %s is not ready", source)
	}

	var domainErr error

	for _, link := range links {
		if err := state.validateLinkDomain(link, vrf); err != nil {
			domainErr = err

			continue
		}

		return nil
	}

	return domainErr
}

func (state *RuntimeState) validateLinkDomain(linkName, wantVRF string) error {
	status, exists := state.linksByName[linkName]
	if !exists {
		return fmt.Errorf("link %q is not ready", linkName)
	}

	actualVRF, err := state.linkVRF(linkName, status)
	if err != nil {
		return err
	}

	if actualVRF != wantVRF {
		if wantVRF == "" {
			return fmt.Errorf("link %q belongs to VRF %q, not the default routing domain", linkName, actualVRF)
		}

		return fmt.Errorf("link %q belongs to VRF %q, not VRF %q", linkName, actualVRF, wantVRF)
	}

	return nil
}

func (state *RuntimeState) linkVRF(name string, status network.LinkStatusSpec) (string, error) {
	seen := map[uint32]struct{}{}

	for {
		if status.Kind == network.LinkKindVRF {
			return name, nil
		}

		masterIndex := status.MasterIndex
		if masterIndex == 0 {
			return "", nil
		}

		if _, exists := seen[masterIndex]; exists {
			return "", fmt.Errorf("link %q has a cyclic master chain", name)
		}

		seen[masterIndex] = struct{}{}

		name, status = state.linkByIndex(masterIndex)
		if name == "" {
			return "", fmt.Errorf("link master index %d is not ready", masterIndex)
		}
	}
}

func (state *RuntimeState) linkByIndex(index uint32) (string, network.LinkStatusSpec) {
	name := state.linkNamesByIndex[index]

	return name, state.linksByName[name]
}

func (state *RuntimeState) advertisedAddresses(links []string) []netip.Prefix {
	linkSet := make(map[string]struct{}, len(links))
	for _, link := range links {
		linkSet[link] = struct{}{}
	}

	if len(linkSet) == 0 {
		return nil
	}

	var addresses []netip.Prefix

	for _, address := range state.addresses {
		if _, ok := linkSet[address.LinkName]; !ok {
			continue
		}

		addr := address.Address.Addr()

		if addr.IsLoopback() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() {
			continue
		}

		addresses = append(addresses, address.Address)
	}

	slices.SortFunc(addresses, func(left, right netip.Prefix) int {
		return left.Addr().Compare(right.Addr())
	})

	return addresses
}

func connectedPrefixes(addresses []netip.Prefix) []netip.Prefix {
	prefixes := make([]netip.Prefix, 0, len(addresses))

	for _, address := range addresses {
		prefixes = append(prefixes, address.Masked())
	}

	slices.SortFunc(prefixes, func(left, right netip.Prefix) int {
		if cmp := left.Addr().Compare(right.Addr()); cmp != 0 {
			return cmp
		}

		return left.Bits() - right.Bits()
	})

	return slices.Compact(prefixes)
}

func routerID(configured netip.Addr, advertised []netip.Prefix) netip.Addr {
	if configured.IsValid() {
		return configured
	}

	for _, prefix := range advertised {
		if prefix.Addr().Is4() {
			return prefix.Addr()
		}
	}

	return netip.Addr{}
}
