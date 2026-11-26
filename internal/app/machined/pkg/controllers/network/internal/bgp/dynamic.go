// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"

	gobgpapi "github.com/osrg/gobgp/v4/api"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const dynamicPeerGroupPrefix = "dynamic/"

// DynamicPeerGroupName returns the GoBGP peer group backing the dynamic neighbors of a prefix.
func DynamicPeerGroupName(prefix netip.Prefix) string {
	return dynamicPeerGroupPrefix + prefix.String()
}

func isDynamicNeighbor(peer *gobgpapi.Peer) bool {
	return strings.HasPrefix(peer.GetConf().GetPeerGroup(), dynamicPeerGroupPrefix) && peer.GetConf().GetNeighborAddress() == ""
}

// DynamicNeighborPeer returns the peer template shared by all dynamic neighbors accepted through a prefix.
func DynamicNeighborPeer(neighbor network.BGPNeighborConfigSpec, vrf string) Peer {
	neighbor.Passive = true

	return Peer{
		Config:        neighbor,
		Address:       neighbor.Prefix.String(),
		BindInterface: vrf,
	}
}

// BuildPeerGroup translates a prefix neighbor into the GoBGP peer group its dynamic neighbors are created from.
func BuildPeerGroup(neighbor network.BGPNeighborConfigSpec, vrf string, multipath bool) *gobgpapi.PeerGroup {
	peer := BuildPeer(DynamicNeighborPeer(neighbor, vrf), multipath)

	return &gobgpapi.PeerGroup{
		Conf: &gobgpapi.PeerGroupConf{
			PeerGroupName: DynamicPeerGroupName(neighbor.Prefix),
			PeerAsn:       peer.Conf.PeerAsn,
			LocalAsn:      peer.Conf.LocalAsn,
			AuthPassword:  peer.Conf.AuthPassword,
		},
		EbgpMultihop:    peer.EbgpMultihop,
		AfiSafis:        peer.AfiSafis,
		Timers:          peer.Timers,
		Transport:       peer.Transport,
		GracefulRestart: peer.GracefulRestart,
		Bfd:             peer.Bfd,
	}
}

// reconcileDynamicNeighbors adds, removes and replaces the peer groups backing prefix neighbors.
//
// GoBGP does not apply peer group updates to the dynamic neighbors already accepted through it,
// so a changed prefix neighbor is recreated, which resets its sessions.
func (instance *Instance) reconcileDynamicNeighbors(ctx context.Context, config network.BGPInstanceConfigSpec) error {
	var errs error

	desired := map[netip.Prefix]network.BGPNeighborConfigSpec{}

	for _, neighbor := range config.Neighbors {
		if neighbor.Prefix.IsValid() {
			desired[neighbor.Prefix] = neighbor
		}
	}

	for prefix, key := range instance.dynamic {
		neighbor, ok := desired[prefix]
		if ok && key == PeerKey(DynamicNeighborPeer(neighbor, config.VRF)) {
			continue
		}

		if err := instance.deleteDynamicNeighbor(ctx, prefix); err != nil {
			errs = errors.Join(errs, err)

			continue
		}

		delete(instance.dynamic, prefix)
	}

	for prefix, neighbor := range desired {
		if _, ok := instance.dynamic[prefix]; ok {
			continue
		}

		if err := instance.addDynamicNeighbor(ctx, neighbor, config.VRF, config.Multipath); err != nil {
			errs = errors.Join(errs, err)

			continue
		}

		instance.dynamic[prefix] = PeerKey(DynamicNeighborPeer(neighbor, config.VRF))
	}

	return errs
}

func (instance *Instance) addDynamicNeighbor(ctx context.Context, neighbor network.BGPNeighborConfigSpec, vrf string, multipath bool) error {
	group := DynamicPeerGroupName(neighbor.Prefix)

	if err := instance.server.AddPeerGroup(ctx, &gobgpapi.AddPeerGroupRequest{PeerGroup: BuildPeerGroup(neighbor, vrf, multipath)}); err != nil {
		return fmt.Errorf("error adding BGP peer group for dynamic neighbors %s: %w", neighbor.Prefix, err)
	}

	if err := instance.server.AddDynamicNeighbor(ctx, &gobgpapi.AddDynamicNeighborRequest{
		DynamicNeighbor: &gobgpapi.DynamicNeighbor{
			Prefix:    neighbor.Prefix.String(),
			PeerGroup: group,
		},
	}); err != nil {
		return errors.Join(
			fmt.Errorf("error adding BGP dynamic neighbors %s: %w", neighbor.Prefix, err),
			instance.server.DeletePeerGroup(ctx, &gobgpapi.DeletePeerGroupRequest{Name: group}),
		)
	}

	return nil
}

// deleteDynamicNeighbor stops accepting sessions from the prefix first, so that no new dynamic neighbor
// joins the peer group while its current members are removed: GoBGP refuses to delete a peer group in use.
func (instance *Instance) deleteDynamicNeighbor(ctx context.Context, prefix netip.Prefix) error {
	group := DynamicPeerGroupName(prefix)

	if err := instance.server.DeleteDynamicNeighbor(ctx, &gobgpapi.DeleteDynamicNeighborRequest{
		Prefix:    prefix.String(),
		PeerGroup: group,
	}); err != nil {
		return fmt.Errorf("error deleting BGP dynamic neighbors %s: %w", prefix, err)
	}

	var members []string

	if err := instance.server.ListPeer(ctx, &gobgpapi.ListPeerRequest{}, func(peer *gobgpapi.Peer) {
		if isDynamicNeighbor(peer) && peer.GetConf().GetPeerGroup() == group {
			members = append(members, peer.GetState().GetNeighborAddress())
		}
	}); err != nil {
		return fmt.Errorf("error listing BGP dynamic neighbors %s: %w", prefix, err)
	}

	var errs error

	for _, address := range members {
		errs = errors.Join(errs, instance.deleteDynamicMember(ctx, address))
	}

	if errs != nil {
		return errs
	}

	if err := instance.server.DeletePeerGroup(ctx, &gobgpapi.DeletePeerGroupRequest{Name: group}); err != nil {
		return fmt.Errorf("error deleting BGP peer group for dynamic neighbors %s: %w", prefix, err)
	}

	return nil
}

// evictDynamicNeighbor deletes the dynamic neighbor accepted from an address, if any,
// so that the neighbor configured with this address can take its place.
func (instance *Instance) evictDynamicNeighbor(ctx context.Context, address string) error {
	var found bool

	if err := instance.server.ListPeer(ctx, &gobgpapi.ListPeerRequest{Address: address}, func(peer *gobgpapi.Peer) {
		found = found || isDynamicNeighbor(peer)
	}); err != nil {
		return fmt.Errorf("error listing BGP peer %s: %w", address, err)
	}

	if !found {
		return nil
	}

	return instance.deleteDynamicMember(ctx, address)
}

// deleteDynamicMember tolerates a dynamic neighbor which is already gone:
// GoBGP deletes a dynamic neighbor by itself when its session goes down.
func (instance *Instance) deleteDynamicMember(ctx context.Context, address string) error {
	err := instance.server.DeletePeer(ctx, &gobgpapi.DeletePeerRequest{Address: address})
	if err == nil {
		return nil
	}

	var exists bool

	if listErr := instance.server.ListPeer(ctx, &gobgpapi.ListPeerRequest{Address: address}, func(*gobgpapi.Peer) {
		exists = true
	}); listErr == nil && !exists {
		return nil
	}

	return fmt.Errorf("error deleting BGP dynamic neighbor %s: %w", address, err)
}
