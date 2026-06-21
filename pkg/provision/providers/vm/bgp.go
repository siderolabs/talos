// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"syscall"

	"github.com/siderolabs/talos/pkg/provision"
)

const (
	bgpPid = "bgp.pid"
	bgpLog = "bgp.log"
)

// CLOSFabricPCIBase is the first PCI slot the full-CLOS fabric NICs are pinned to (qemu `addr=0x10`),
// chosen so the guest kernel interface names are deterministic.
const CLOSFabricPCIBase = 0x10

// CLOSFabricIfaceName returns the predictable guest kernel interface name for fabric uplink i on a
// full-CLOS node (no net0), pinned to PCI slot CLOSFabricPCIBase+i. Used for the talos.config link-local
// zone and the baked BGPConfig neighbor names. NOTE: confirm on the first live boot (machine-type/arch).
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

// CreateBGP creates an embedded gobgp fabric peer for testing native BGP.
func (p *Provisioner) CreateBGP(state *provision.State, clusterReq provision.ClusterRequest, options provision.Options) error {
	pidPath := state.GetRelativePath(bgpPid)

	logFile, err := os.OpenFile(state.GetRelativePath(bgpLog), os.O_APPEND|os.O_CREATE|os.O_RDWR, 0o666)
	if err != nil {
		return err
	}

	defer logFile.Close() //nolint:errcheck

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

	if options.BGPCLOS {
		// full-CLOS: nodes have no net0, so they are reachable only via BGP. Peer unnumbered over every
		// node's dedicated fabric uplink(s), program their learned loopback /32s into the host FIB (zebra),
		// and NAT the loopback CIDR so the nodes reach host services + the internet. The bridge names match
		// what the node launcher uses.
		args = append(args, "--bgp-unnumbered", "--bgp-zebra")

		if options.BGPLoopbackCIDR != "" {
			args = append(args, "--bgp-nat-cidr", options.BGPLoopbackCIDR)
		}

		for i := range clusterReq.Nodes {
			for u := range clusterReq.Network.FabricUplinks {
				args = append(args, "--bgp-interface", FabricBridgeName(clusterReq.Network.Name, i, u))
			}
		}
	}

	cmd := exec.Command(clusterReq.SelfExecutable, args...) //nolint:noctx // runs in background
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true, // daemonize
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	if err = os.WriteFile(pidPath, []byte(strconv.Itoa(cmd.Process.Pid)), os.ModePerm); err != nil {
		return fmt.Errorf("error writing BGP PID file: %w", err)
	}

	return nil
}

// DestroyBGP destroys the embedded gobgp fabric peer.
func (p *Provisioner) DestroyBGP(state *provision.State) error {
	pidPath := state.GetRelativePath(bgpPid)

	return StopProcessByPidfile(pidPath)
}
