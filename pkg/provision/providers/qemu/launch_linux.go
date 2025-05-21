// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/netip"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/alexflint/go-filemutex"
	"github.com/containernetworking/cni/libcni"
	"github.com/containernetworking/cni/pkg/types"
	types100 "github.com/containernetworking/cni/pkg/types/100"
	"github.com/containernetworking/plugins/pkg/ns"
	"github.com/containernetworking/plugins/pkg/testutils"
	"github.com/containernetworking/plugins/pkg/utils"
	"github.com/coreos/go-iptables/iptables"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/xslices"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/internal/cniutils"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

type networkConfig struct {
	networkConfigBase

	// TODO: rename field to cniNetworkConfig
	CniNetworkConfig  *libcni.NetworkConfigList
	CNI               provision.CNIConfig
	NoMasqueradeCIDRs []netip.Prefix

	// filled by CNI invocation
	tapName string
	ns      ns.NetNS
}

func getLaunchNetworkConfig(state *vm.State, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest) networkConfig {
	return networkConfig{
		networkConfigBase: getLaunchNetworkConfigBase(state, clusterReq, nodeReq),
		CniNetworkConfig:  state.VMCNIConfig,
		CNI:               clusterReq.Network.CNI,
		NoMasqueradeCIDRs: clusterReq.Network.NoMasqueradeCIDRs,
	}
}

func getNetdevParams(networkConfig networkConfig, id string) string {
	return fmt.Sprintf("tap,id=%s,ifname=%s,script=no,downscript=no", id, networkConfig.tapName)
}

func getConfigServerAddr(hostAddrs net.Addr, _ LaunchConfig) (net.Addr, error) {
	return hostAddrs, nil
}

// withCNIOperationLocked ensures that CNI operations don't run concurrently.
//
// There are race conditions in the CNI plugins that can cause a failure if called concurrently.
func withCNIOperationLocked[T any](config *LaunchConfig, f func() (T, error)) (T, error) {
	var zeroT T

	lock, err := filemutex.New(filepath.Join(config.StatePath, "cni.lock"))
	if err != nil {
		return zeroT, fmt.Errorf("failed to create CNI lock: %w", err)
	}

	if err = lock.Lock(); err != nil {
		return zeroT, fmt.Errorf("failed to acquire CNI lock: %w", err)
	}

	defer func() {
		if err := lock.Close(); err != nil {
			log.Printf("failed to release CNI lock: %s", err)
		}
	}()

	return f()
}

// withCNIOperationLockedNoResult ensures that CNI operations don't run concurrently.
func withCNIOperationLockedNoResult(config *LaunchConfig, f func() error) error {
	_, err := withCNIOperationLocked(config, func() (struct{}, error) {
		return struct{}{}, f()
	})

	return err
}

// withNetworkContext creates a network namespace, launches CNI and passes control to the next function
// filling config with netNS and interface details.
//
//nolint:gocyclo
func withNetworkContext(ctx context.Context, config *LaunchConfig, f func(config *LaunchConfig) error) error {
	// random ID for the CNI, maps to single VM
	containerID := uuid.New().String()

	cniConfig := libcni.NewCNIConfigWithCacheDir(config.Network.CNI.BinPath, config.Network.CNI.CacheDir, nil)

	// create a network namespace
	ns, err := testutils.NewNS()
	if err != nil {
		return err
	}

	defer func() {
		ns.Close()              //nolint:errcheck
		testutils.UnmountNS(ns) //nolint:errcheck
	}()

	ips := make([]string, len(config.Network.IPs))
	for j := range ips {
		ips[j] = sideronet.FormatCIDR(config.Network.IPs[j], config.Network.CIDRs[j])
	}

	gatewayAddrs := xslices.Map(config.Network.GatewayAddrs, netip.Addr.String)

	runtimeConf := libcni.RuntimeConf{
		ContainerID: containerID,
		NetNS:       ns.Path(),
		IfName:      "veth0",
		Args: [][2]string{
			{"IP", strings.Join(ips, ",")},
			{"GATEWAY", strings.Join(gatewayAddrs, ",")},
			{"IgnoreUnknown", "1"},
		},
	}

	// attempt to clean up network in case it was deployed previously
	err = withCNIOperationLockedNoResult(
		config,
		func() error {
			return cniConfig.DelNetworkList(ctx, config.Network.CniNetworkConfig, &runtimeConf)
		},
	)
	if err != nil {
		return fmt.Errorf("error deleting CNI network: %w", err)
	}

	res, err := withCNIOperationLocked(
		config,
		func() (types.Result, error) {
			return cniConfig.AddNetworkList(ctx, config.Network.CniNetworkConfig, &runtimeConf)
		},
	)
	if err != nil {
		return fmt.Errorf("error provisioning CNI network: %w", err)
	}

	defer func() {
		if e := withCNIOperationLockedNoResult(
			config,
			func() error {
				return cniConfig.DelNetworkList(ctx, config.Network.CniNetworkConfig, &runtimeConf)
			},
		); e != nil {
			log.Printf("error cleaning up CNI: %s", e)
		}
	}()

	currentResult, err := types100.NewResultFromResult(res)
	if err != nil {
		return fmt.Errorf("failed to parse cni result: %w", err)
	}

	vmIface, tapIface, err := cniutils.VMTapPair(currentResult, containerID)
	if err != nil {
		return errors.New(
			"failed to parse VM network configuration from CNI output, ensure CNI is configured with a plugin " +
				"that supports automatic VM network configuration such as tc-redirect-tap")
	}

	cniChain := utils.FormatChainName(config.Network.CniNetworkConfig.Name, containerID)

	ipt, err := iptables.New()
	if err != nil {
		return fmt.Errorf("failed to initialize iptables: %w", err)
	}

	// don't masquerade traffic with "broadcast" destination from the VM
	//
	// no need to clean up the rule, as CNI drops the whole chain
	if err = ipt.InsertUnique("nat", cniChain, 1, "--destination", "255.255.255.255/32", "-j", "ACCEPT"); err != nil {
		return fmt.Errorf("failed to insert iptables rule to allow broadcast traffic: %w", err)
	}

	for _, cidr := range config.Network.NoMasqueradeCIDRs {
		if err = ipt.InsertUnique("nat", cniChain, 1, "--destination", cidr.String(), "-j", "ACCEPT"); err != nil {
			return fmt.Errorf("failed to insert iptables rule to allow non-masquerade traffic to cidr %q: %w", cidr.String(), err)
		}
	}

	config.Network.tapName = tapIface.Name
	config.VMMac = vmIface.Mac
	config.Network.ns = ns

	return f(config)
}

func startQemuCmd(config *LaunchConfig, cmd *exec.Cmd) error {
	if err := ns.WithNetNSPath(config.Network.ns.Path(), func(_ ns.NetNS) error {
		return cmd.Start()
	}); err != nil {
		return err
	}

	return nil
}
