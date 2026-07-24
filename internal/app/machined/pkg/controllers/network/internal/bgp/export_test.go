// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"maps"
	"net/netip"

	"github.com/osrg/gobgp/v4/pkg/apiutil"
	bgppacket "github.com/osrg/gobgp/v4/pkg/packet/bgp"
	gobgpsrv "github.com/osrg/gobgp/v4/pkg/server"
)

// InstanceServerForTest exposes the current GoBGP server identity.
func InstanceServerForTest(instance *Instance) *gobgpsrv.BgpServer {
	return instance.server
}

// SetInstanceServerForTest replaces the server used by an instance.
func SetInstanceServerForTest(instance *Instance, server *gobgpsrv.BgpServer) {
	instance.server = server
}

// InstanceMapsInitializedForTest reports whether constructor-owned maps are initialized.
func InstanceMapsInitializedForTest(instance *Instance) bool {
	return instance.originated != nil && instance.imported != nil && instance.peers != nil && instance.peerIfaces != nil
}

// InstancePeerKeysForTest returns the reconciled peer-key snapshot.
func InstancePeerKeysForTest(instance *Instance) map[string]string {
	return maps.Clone(instance.peers)
}

// InstancePeerIfacesForTest returns the learned link-local peer interface mapping.
func InstancePeerIfacesForTest(instance *Instance) map[netip.Addr]string {
	return maps.Clone(instance.peerIfaces)
}

// InstanceOriginatedForTest reports whether a prefix is currently originated.
func InstanceOriginatedForTest(instance *Instance, prefix netip.Prefix) bool {
	_, ok := instance.originated[prefix]

	return ok
}

// InstanceImportedForTest reports whether a prefix is currently imported.
func InstanceImportedForTest(instance *Instance, prefix netip.Prefix) bool {
	_, ok := instance.imported[prefix]

	return ok
}

// ImportFamiliesForTest exposes import selector family selection.
func ImportFamiliesForTest(selectors []netip.Prefix) []bgppacket.Family {
	return importFamilies(selectors)
}

// ListImportCandidatesForTest exposes candidate selection for lifecycle tests.
func ListImportCandidatesForTest(instance *Instance, selectors []netip.Prefix) (map[netip.Prefix]*apiutil.Path, error) {
	return instance.listImportCandidates(selectors)
}
