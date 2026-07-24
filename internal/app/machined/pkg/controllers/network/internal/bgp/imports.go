// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"fmt"
	"net/netip"
	"slices"

	"github.com/google/uuid"
	gobgpapi "github.com/osrg/gobgp/v4/api"
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

	for _, family := range importFamilies(selectors) {
		err := instance.server.ListPath(apiutil.ListPathRequest{
			TableType: gobgpapi.TableType_TABLE_TYPE_GLOBAL,
			Family:    family,
		}, func(prefix bgppacket.NLRI, paths []*apiutil.Path) {
			addImportCandidates(candidates, selectors, prefix, paths)
		})
		if err != nil {
			return nil, fmt.Errorf("error listing %s paths: %w", family, err)
		}
	}

	return candidates, nil
}

func (instance *Instance) reconcileImported(desired map[netip.Prefix]desiredImportedPath) error {
	for prefix, current := range instance.imported {
		path, exists := desired[prefix]
		if exists && path.fingerprint == current.fingerprint {
			continue
		}

		if err := instance.server.DeletePath(apiutil.DeletePathRequest{UUIDs: []uuid.UUID{current.uuid}}); err != nil {
			return fmt.Errorf("error deleting imported path %s: %w", prefix, err)
		}

		delete(instance.imported, prefix)
	}

	for prefix, imported := range desired {
		if _, exists := instance.imported[prefix]; exists {
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

func importFamilies(selectors []netip.Prefix) []bgppacket.Family {
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

func addImportCandidates(
	candidates map[netip.Prefix]*apiutil.Path,
	selectors []netip.Prefix,
	prefix bgppacket.NLRI,
	paths []*apiutil.Path,
) {
	dst, err := netip.ParsePrefix(prefix.String())
	if err != nil || !matchesImportSelector(dst, selectors) {
		return
	}

	for _, path := range paths {
		if !path.Best || path.Withdrawal || !path.PeerAddress.IsValid() {
			continue
		}

		current := candidates[dst]
		if current == nil || path.PeerAddress.Compare(current.PeerAddress) < 0 {
			candidates[dst] = path
		}
	}
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
