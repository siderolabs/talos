// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package clustermaker

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/netip"
	"os"

	"github.com/siderolabs/go-kubeconfig"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/provision/access"
)

const (
	// gatewayOffset is the offset from the network address of the IP address of the network gateway.
	gatewayOffset = 1

	// nodesOffset is the offset from the network address of the beginning of the IP addresses to be used for nodes.
	nodesOffset  = 2
	jsonLogsPort = 4003
)

func patchWireguard(wireguardConfigBundle *helpers.WireguardConfigBundle, cfg config.Provider, nodeIPs []netip.Addr) (config.Provider, error) {
	if wireguardConfigBundle != nil {
		return wireguardConfigBundle.PatchConfig(nodeIPs[0], cfg)
	}

	return cfg, nil
}

func saveConfig(talosConfigObj *clientconfig.Config, commonOps Options) (err error) {
	c, err := clientconfig.Open(commonOps.Talosconfig)
	if err != nil {
		return fmt.Errorf("error opening talos config: %w", err)
	}

	renames := c.Merge(talosConfigObj)
	for _, rename := range renames {
		fmt.Fprintf(os.Stderr, "renamed talosconfig context %s\n", rename.String())
	}

	return c.Save(commonOps.Talosconfig)
}

func mergeKubeconfig(ctx context.Context, clusterAccess *access.Adapter) error {
	kubeconfigPath, err := kubeconfig.DefaultPath()
	if err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "\nmerging kubeconfig into %q\n", kubeconfigPath)

	k8sconfig, err := clusterAccess.Kubeconfig(ctx)
	if err != nil {
		return fmt.Errorf("error fetching kubeconfig: %w", err)
	}

	kubeConfig, err := clientcmd.Load(k8sconfig)
	if err != nil {
		return fmt.Errorf("error parsing kubeconfig: %w", err)
	}

	if clusterAccess.ForceEndpoint != "" {
		for name := range kubeConfig.Clusters {
			kubeConfig.Clusters[name].Server = clusterAccess.ForceEndpoint
		}
	}

	_, err = os.Stat(kubeconfigPath)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		return clientcmd.WriteToFile(*kubeConfig, kubeconfigPath)
	}

	merger, err := kubeconfig.Load(kubeconfigPath)
	if err != nil {
		return fmt.Errorf("error loading existing kubeconfig: %w", err)
	}

	err = merger.Merge(kubeConfig, kubeconfig.MergeOptions{
		ActivateContext: true,
		OutputWriter:    os.Stdout,
		ConflictHandler: func(component kubeconfig.ConfigComponent, name string) (kubeconfig.ConflictDecision, error) {
			return kubeconfig.RenameDecision, nil
		},
	})
	if err != nil {
		return fmt.Errorf("error merging kubeconfig: %w", err)
	}

	return merger.Write(kubeconfigPath)
}

func parseCPUShare(cpus string) (int64, error) {
	cpu, ok := new(big.Rat).SetString(cpus)
	if !ok {
		return 0, fmt.Errorf("failed to parsing as a rational number: %s", cpus)
	}

	nano := cpu.Mul(cpu, big.NewRat(1e9, 1))
	if !nano.IsInt() {
		return 0, errors.New("value is too precise")
	}

	return nano.Num().Int64(), nil
}

func getNodeIP(cidrs []netip.Prefix, ips [][]netip.Addr, nodeIndex int) []netip.Addr {
	nodeIPs := make([]netip.Addr, len(cidrs))
	for j := range nodeIPs {
		nodeIPs[j] = ips[j][nodeIndex]
	}

	return nodeIPs
}
