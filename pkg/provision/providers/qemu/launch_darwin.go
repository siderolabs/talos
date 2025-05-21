// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"os/exec"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

type networkConfig struct {
	networkConfigBase
	StartAddr netip.Addr
	EndAddr   netip.Addr
}

func getLaunchNetworkConfig(state *vm.State, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest) networkConfig {
	// This ip will be assigned to the bridge
	// The following ips will be assigned to the vms
	startAddr := clusterReq.Nodes[0].IPs[0].Prev()
	endAddr := clusterReq.Nodes[len(clusterReq.Nodes)-1].IPs[0].Next()

	return networkConfig{
		networkConfigBase: getLaunchNetworkConfigBase(state, clusterReq, nodeReq),
		StartAddr:         startAddr,
		EndAddr:           endAddr,
	}
}

func getNetdevParams(networkConfig networkConfig, id string) string {
	netDevArg := "vmnet-shared,id=" + id
	cidr := networkConfig.CIDRs[0]
	m := net.CIDRMask(cidr.Bits(), 32)
	subnetMask := fmt.Sprintf("%d.%d.%d.%d", m[0], m[1], m[2], m[3])
	netDevArg += fmt.Sprintf(",start-address=%s,end-address=%s,subnet-mask=%s", networkConfig.StartAddr, networkConfig.EndAddr, subnetMask)

	return netDevArg
}

// getConfigServerAddr returns the ip accessible to the VM that will route to the config server.
// hostAddrs is the address on which the server is accessible from the host network.
func getConfigServerAddr(hostAddrs net.Addr, config LaunchConfig) (netip.AddrPort, error) {
	addrPort, err := netip.ParseAddrPort(hostAddrs.String())
	if err != nil {
		return netip.AddrPort{}, err
	}

	return netip.AddrPortFrom(config.Network.GatewayAddrs[0], addrPort.Port()), nil
}

// withNetworkContext runs the f on the host network on darwin.
func withNetworkContext(ctx context.Context, config *LaunchConfig, f func(config *LaunchConfig) error) error {
	return f(config)
}

// startQemuCmd on darwin just runs cmd.Start.
func startQemuCmd(_ *LaunchConfig, cmd *exec.Cmd) error {
	return cmd.Start()
}
