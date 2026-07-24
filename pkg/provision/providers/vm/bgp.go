// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"strconv"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	bgpPid = "bgp.pid"
	bgpLog = "bgp.log"
)

// CLOSFabricPCIBase is the first PCI slot the full-CLOS fabric NICs are pinned to (qemu `addr=0x10`),
// chosen so the guest kernel interface names are deterministic.
const CLOSFabricPCIBase = 0x10

var bgpVRFPeerPrefix = netip.MustParsePrefix("192.0.2.2/30")

// CLOSFabricIfaceName returns the predictable guest kernel interface name for fabric uplink i on a
// full-CLOS node (no net0), pinned to PCI slot CLOSFabricPCIBase+i. Used for the talos.config link-local
// zone and the baked BGPInstanceConfig neighbor names. NOTE: confirm on the first live boot (machine-type/arch).
func CLOSFabricIfaceName(i int) string {
	return fmt.Sprintf("enp0s%d", CLOSFabricPCIBase+i)
}

// FabricBridgeName returns the deterministic host bridge name for a node's BGP fabric uplink. It is
// computed identically by the node launcher (which attaches the uplink) and by CreateBGP (which tells
// the fabric peer which interfaces to peer on). Bounded to <=15 chars (Linux interface name limit).
func FabricBridgeName(networkName string, nodeIdx, uplinkIdx int) string {
	h := sha256.Sum256(fmt.Appendf(nil, "%s-%d-%d", networkName, nodeIdx, uplinkIdx))

	return "bgp" + hex.EncodeToString(h[:])[:11]
}

// VRFPeerPrefix returns the isolated guest prefix used by the inbound VRF BGP listener test.
func VRFPeerPrefix() netip.Prefix {
	return bgpVRFPeerPrefix
}

// VRFPeerAddress returns the isolated guest address used by the inbound VRF BGP listener test.
func VRFPeerAddress() netip.Addr {
	return bgpVRFPeerPrefix.Addr()
}

// CreateBGP creates an embedded gobgp fabric peer for testing native BGP.
func (p *Provisioner) CreateBGP(state *provision.State, clusterReq provision.ClusterRequest, options provision.Options) error {
	pidPath := state.GetRelativePath(bgpPid)

	logFile, err := os.OpenFile(state.GetRelativePath(bgpLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

	args := bgpLaunchArgs(clusterReq, options)

	if err = prepareBGPVRFPeerRoute(state, clusterReq, options); err != nil {
		return err
	}

	cmd := exec.Command(clusterReq.SelfExecutable, args...) //nolint:noctx // runs in background
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	setDetachedProcess(cmd)

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return fmt.Errorf("error writing BGP PID file: %w", err)
	}

	return nil
}

func bgpLaunchArgs(clusterReq provision.ClusterRequest, options provision.Options) []string {
	listenAddr := options.BGPListenAddress
	if listenAddr == "" && len(clusterReq.Network.GatewayAddrs) > 0 {
		// unnumbered mode has no configured listen address; use the bridge gateway as the router-id.
		listenAddr = clusterReq.Network.GatewayAddrs[0].String()
	}

	args := []string{
		"bgp-launch",
		"--bgp-addr", listenAddr,
		"--bgp-neighbor-range", options.BGPNeighborRange,
		"--bgp-advertise", options.BGPAdvertise,
		"--bgp-asn", strconv.FormatUint(uint64(options.BGPLocalASN), 10),
		"--bgp-peer-asn", strconv.FormatUint(uint64(options.BGPPeerASN), 10),
	}

	if !options.BGPCLOS {
		return append(args, "--bgp-vrf-neighbor", VRFPeerAddress().String())
	}

	// Full CLOS: nodes have no net0, so they are reachable only via BGP. Peer unnumbered over every
	// node fabric uplink, program learned loopbacks into the host FIB, and NAT the loopback CIDR.
	args = append(args, "--bgp-unnumbered", "--bgp-zebra")

	if options.BGPLoopbackCIDR != "" {
		args = append(args, "--bgp-nat-cidr", options.BGPLoopbackCIDR)
	}

	for i := range clusterReq.Nodes {
		for u := range clusterReq.Network.FabricUplinks {
			args = append(args, "--bgp-interface", FabricBridgeName(clusterReq.Network.Name, i, u))
		}
	}

	return args
}

func prepareBGPVRFPeerRoute(state *provision.State, clusterReq provision.ClusterRequest, options provision.Options) error {
	if options.BGPCLOS {
		return nil
	}

	if len(clusterReq.Network.GatewayAddrs) == 0 {
		return fmt.Errorf("BGP VRF peer route requires a bridge gateway address")
	}

	return configureBGPVRFPeerRoute(state.BridgeName, VRFPeerAddress(), clusterReq.Network.GatewayAddrs[0])
}

// DestroyBGP destroys the embedded gobgp fabric peer.
func (p *Provisioner) DestroyBGP(state *provision.State) error {
	pidPath := state.GetRelativePath(bgpPid)

	return StopProcessByPidfile(pidPath)
}
