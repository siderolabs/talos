// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"fmt"
	"iter"
	"net/netip"
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ResolvedConfig is a BGP instance configuration resolved against current network runtime state.
type ResolvedConfig struct {
	Spec               network.BGPInstanceConfigSpec
	AdvertisedPrefixes []netip.Prefix
	// ConnectedPrefixes are the prefixes of all addresses in the instance's routing domain.
	//
	// The kernel already routes them on-link, and a learned route for the same prefix would replace
	// that route, as Talos installs address routes with a higher metric than BGP routes.
	ConnectedPrefixes []netip.Prefix
	RouterID          netip.Addr
	// RouterIDCandidates are the IPv4 addresses on the advertised links, lowest first: a derived router ID
	// is the first of them, and a running instance may keep any of them.
	RouterIDCandidates []netip.Addr
	// VRFIndex is the kernel index of the VRF link, zero when the instance runs in the main table.
	//
	// The listening socket is bound to the device by name, which the kernel resolves to an index
	// once. A VRF recreated under the same name gets a new index, so the index is part of the
	// server's identity: without it a rebuilt VRF leaves the server bound to a device which no
	// longer exists, and it neither accepts nor sources connections in the new VRF.
	VRFIndex uint32
}

// RuntimeState is an indexed snapshot of network link and address status.
type RuntimeState struct {
	linkResolver     *network.LinkResolver
	linksByName      map[string]network.LinkStatusSpec
	linkNamesByIndex map[uint32]string
	addressLinks     map[netip.Addr][]string
	addressesByLink  map[uint32]map[netip.Addr]struct{}
	addresses        []network.AddressStatusSpec
}

// NewRuntimeState builds an indexed network runtime snapshot from status resources.
func NewRuntimeState(links func() iter.Seq[*network.LinkStatus], addresses iter.Seq[*network.AddressStatus]) *RuntimeState {
	state := &RuntimeState{
		linkResolver:     network.NewLinkResolver(links),
		linksByName:      map[string]network.LinkStatusSpec{},
		linkNamesByIndex: map[uint32]string{},
		addressLinks:     map[netip.Addr][]string{},
		addressesByLink:  map[uint32]map[netip.Addr]struct{}{},
	}

	for status := range links() {
		name := status.Metadata().ID()

		state.linksByName[name] = *status.TypedSpec()
		state.linkNamesByIndex[status.TypedSpec().Index] = name
	}

	for status := range addresses {
		spec := *status.TypedSpec()
		state.addresses = append(state.addresses, spec)

		address := spec.Address.Addr()
		linkName := state.linkResolver.Resolve(spec.LinkName)

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
	resolved.VRF = state.linkResolver.Resolve(spec.VRF)
	resolved.AdvertiseLinks = slices.Clone(spec.AdvertiseLinks)
	resolved.Neighbors = slices.Clone(spec.Neighbors)

	var vrfIndex uint32

	if resolved.VRF != "" {
		status, exists := state.linksByName[resolved.VRF]
		if !exists {
			return ResolvedConfig{}, fmt.Errorf("VRF link %q is not ready", resolved.VRF)
		}

		if status.Kind != network.LinkKindVRF {
			return ResolvedConfig{}, fmt.Errorf("link %q is not a VRF", resolved.VRF)
		}

		vrfIndex = status.Index
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
	routerIDCandidates := routerIDCandidates(advertisedAddresses)

	return ResolvedConfig{
		Spec:               resolved,
		AdvertisedPrefixes: connectedPrefixes(advertisedAddresses),
		ConnectedPrefixes:  connectedPrefixes(state.domainAddresses(resolved.VRF)),
		RouterID:           selectRouterID(resolved.RouterID, routerIDCandidates),
		RouterIDCandidates: routerIDCandidates,
		VRFIndex:           vrfIndex,
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

func (state *RuntimeState) resolveLinks(resolved *network.BGPInstanceConfigSpec) error {
	for i, link := range resolved.AdvertiseLinks {
		resolved.AdvertiseLinks[i] = state.linkResolver.Resolve(link)

		if err := state.validateLinkDomain(resolved.AdvertiseLinks[i], resolved.VRF); err != nil {
			return fmt.Errorf("advertised link: %w", err)
		}
	}

	for i := range resolved.Neighbors {
		if resolved.Neighbors[i].Link == "" {
			continue
		}

		resolved.Neighbors[i].Link = state.linkResolver.Resolve(resolved.Neighbors[i].Link)

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

func (state *RuntimeState) domainAddresses(vrf string) []netip.Prefix {
	var addresses []netip.Prefix

	for _, address := range state.addresses {
		linkName := state.linkResolver.Resolve(address.LinkName)

		status, exists := state.linksByName[linkName]
		if !exists {
			continue
		}

		if linkVRF, err := state.linkVRF(linkName, status); err != nil || linkVRF != vrf {
			continue
		}

		addresses = append(addresses, address.Address)
	}

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

func selectRouterID(configured netip.Addr, candidates []netip.Addr) netip.Addr {
	if configured.IsValid() || len(candidates) == 0 {
		return configured
	}

	return candidates[0]
}

func routerIDCandidates(advertised []netip.Prefix) []netip.Addr {
	var candidates []netip.Addr

	for _, prefix := range advertised {
		if prefix.Addr().Is4() {
			candidates = append(candidates, prefix.Addr())
		}
	}

	return slices.Compact(candidates)
}
