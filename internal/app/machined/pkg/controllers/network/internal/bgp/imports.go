// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/google/uuid"
	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type importedPath struct {
	uuid        uuid.UUID
	fingerprint [32]byte
}

type desiredImportedPath struct {
	path        *apiutil.Path
	fingerprint [32]byte
}

func newImportedPath(source *apiutil.Path) (desiredImportedPath, error) {
	path, err := BuildImportedPath(source)
	if err != nil {
		return desiredImportedPath{}, err
	}

	fingerprint, err := PathFingerprint(path)
	if err != nil {
		return desiredImportedPath{}, err
	}

	return desiredImportedPath{path: path, fingerprint: [32]byte(fingerprint)}, nil
}

// ReconcileImportedPaths resolves and reconciles selected routes from other BGP instances.
func ReconcileImportedPaths(
	targetName string,
	target *Instance,
	instances map[string]*Instance,
	desiredInstances map[string]struct{},
) (int, bool, error) {
	desired, ready, err := resolveImportedPaths(targetName, target, instances, desiredInstances)
	if err != nil || !ready {
		return 0, ready, err
	}

	if err = target.reconcileImported(desired); err != nil {
		return 0, false, err
	}

	return len(desired), true, nil
}

func resolveImportedPaths(
	targetName string,
	target *Instance,
	instances map[string]*Instance,
	desiredInstances map[string]struct{},
) (map[netip.Prefix]desiredImportedPath, bool, error) {
	desired := map[netip.Prefix]desiredImportedPath{}

	for _, routeImport := range target.imports {
		if _, configured := desiredInstances[routeImport.BGPInstance]; !configured {
			continue
		}

		source := instances[routeImport.BGPInstance]
		if source == nil || !source.Running() {
			return nil, false, nil
		}

		candidates, err := source.listImportCandidates(routeImport.Prefixes)
		if err != nil {
			return nil, false, fmt.Errorf("source BGP instance %q: %w", routeImport.BGPInstance, err)
		}

		for prefix, sourcePath := range candidates {
			if _, originated := target.originated[prefix]; originated {
				continue
			}

			if _, duplicate := desired[prefix]; duplicate {
				return nil, false, fmt.Errorf("prefix %s is selected from more than one source for target %q", prefix, targetName)
			}

			path, err := newImportedPath(sourcePath)
			if err != nil {
				return nil, false, fmt.Errorf("prefix %s from source %q: %w", prefix, routeImport.BGPInstance, err)
			}

			desired[prefix] = path
		}
	}

	return desired, true, nil
}

// InstallRoutes reports whether learned routes should be installed into the kernel.
func (instance *Instance) InstallRoutes() bool {
	return instance.installRoutes
}

// ImportedCount returns the number of paths currently imported into the instance.
func (instance *Instance) ImportedCount() int {
	return len(instance.imported)
}

func (instance *Instance) listImportCandidates(selectors []netip.Prefix) (map[netip.Prefix]*apiutil.Path, error) {
	candidates := map[netip.Prefix]*apiutil.Path{}

	for _, family := range ImportFamilies(selectors) {
		if err := instance.listPaths(family, func(prefix netip.Prefix, paths []*apiutil.Path) {
			if candidate := ImportCandidate(prefix, paths, selectors); candidate != nil {
				candidates[prefix] = candidate
			}
		}); err != nil {
			return nil, fmt.Errorf("error listing %s paths: %w", family, err)
		}
	}

	return candidates, nil
}

func (instance *Instance) reconcileImported(desired map[netip.Prefix]desiredImportedPath) error {
	for prefix, current := range instance.imported {
		if _, exists := desired[prefix]; exists {
			continue
		}

		if err := instance.server.DeletePath(apiutil.DeletePathRequest{UUIDs: []uuid.UUID{current.uuid}}); err != nil {
			return fmt.Errorf("error deleting imported path %s: %w", prefix, err)
		}

		delete(instance.imported, prefix)
	}

	// a changed path is added over the current one: GoBGP keys local paths by prefix, so AddPath replaces
	// it in place under a new UUID, while deleting it first would send a withdrawal to the peers
	for prefix, imported := range desired {
		if current, exists := instance.imported[prefix]; exists && current.fingerprint == imported.fingerprint {
			continue
		}

		responses, err := instance.server.AddPath(apiutil.AddPathRequest{Paths: []*apiutil.Path{imported.path}})
		if err != nil {
			return fmt.Errorf("error adding imported path %s: %w", prefix, err)
		}

		if len(responses) != 1 {
			return fmt.Errorf("adding imported path %s returned %d responses", prefix, len(responses))
		}

		if responses[0].Error != nil {
			return fmt.Errorf("error adding imported path %s: %w", prefix, responses[0].Error)
		}

		instance.imported[prefix] = importedPath{
			uuid:        responses[0].UUID,
			fingerprint: imported.fingerprint,
		}
	}

	return nil
}

func cloneImportRoutes(imports []network.BGPImportRouteSpec) []network.BGPImportRouteSpec {
	result := make([]network.BGPImportRouteSpec, len(imports))

	for i := range imports {
		result[i] = imports[i]
		result[i].Prefixes = slices.Clone(imports[i].Prefixes)
	}

	return result
}

// ImportFamilies returns the address families to list for import selectors.
func ImportFamilies(selectors []netip.Prefix) []bgppacket.Family {
	var hasIPv4, hasIPv6 bool

	for _, selector := range selectors {
		if selector.Addr().Is4() {
			hasIPv4 = true
		} else if selector.Addr().Is6() {
			hasIPv6 = true
		}
	}

	families := make([]bgppacket.Family, 0, 2)
	if hasIPv4 {
		families = append(families, bgppacket.RF_IPv4_UC)
	}

	if hasIPv6 {
		families = append(families, bgppacket.RF_IPv6_UC)
	}

	return families
}

// ImportCandidate returns the path of a destination to import, or nil if the prefix does not match the selectors:
// the best path learned from a neighbor, from the lowest peer address among several. Local paths, originated
// or imported themselves, are never candidates, so imports do not chain.
func ImportCandidate(prefix netip.Prefix, paths []*apiutil.Path, selectors []netip.Prefix) *apiutil.Path {
	if !matchesImportSelector(prefix, selectors) {
		return nil
	}

	var candidate *apiutil.Path

	for _, path := range paths {
		if !path.Best || path.Withdrawal || !path.PeerAddress.IsValid() {
			continue
		}

		if candidate == nil || path.PeerAddress.Compare(candidate.PeerAddress) < 0 {
			candidate = path
		}
	}

	return candidate
}

func matchesImportSelector(prefix netip.Prefix, selectors []netip.Prefix) bool {
	for _, selector := range selectors {
		if selector.Addr().BitLen() == prefix.Addr().BitLen() &&
			prefix.Bits() >= selector.Bits() &&
			selector.Contains(prefix.Addr()) {
			return true
		}
	}

	return false
}
