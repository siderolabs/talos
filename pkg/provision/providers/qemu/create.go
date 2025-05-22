// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// Create Talos cluster as a set of qemu VMs.
//
//nolint:gocyclo,cyclop
func (p *provisioner) Create(ctx context.Context, request provision.ClusterRequest, opts ...provision.Option) (provision.Cluster, error) {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	arch := Arch(options.TargetArch)
	if err := arch.Valid(); err != nil {
		return nil, err
	}

	if err := p.preflightChecks(ctx, request, options, arch); err != nil {
		return nil, err
	}

	statePath := filepath.Join(request.StateDirectory, request.Name)

	fmt.Fprintf(options.LogWriter, "creating state directory in %q\n", statePath)

	state, err := vm.NewState(
		statePath,
		p.Name,
		request.Name,
	)
	if err != nil {
		return nil, err
	}

	if options.SiderolinkEnabled {
		fmt.Fprintln(options.LogWriter, "creating siderolink agent")

		if err = p.CreateSiderolinkAgent(state, request); err != nil {
			return nil, err
		}

		fmt.Fprintln(options.LogWriter, "created siderolink agent")
	}

	fmt.Fprintln(options.LogWriter, "creating network", request.Network.Name)

	if err = p.CreateNetwork(ctx, state, request.Network, options); err != nil {
		return nil, fmt.Errorf("unable to provision CNI network: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "creating load balancer")

	if err = p.CreateLoadBalancer(state, request); err != nil {
		return nil, fmt.Errorf("error creating loadbalancer: %w", err)
	}

	if options.KMSEndpoint != "" {
		fmt.Fprintln(options.LogWriter, "creating KMS server")

		if err = p.CreateKMS(state, request, options); err != nil {
			return nil, fmt.Errorf("error creating KMS server: %w", err)
		}
	}

	if options.JSONLogsEndpoint != "" {
		fmt.Fprintln(options.LogWriter, "creating JSON logs server")

		if err = p.CreateJSONLogs(state, request, options); err != nil {
			return nil, fmt.Errorf("error creating JSON logs server: %w", err)
		}
	}

	var nodeInfo []provision.NodeInfo

	fmt.Fprintln(options.LogWriter, "creating controlplane nodes")

	if nodeInfo, err = p.createNodes(state, request, request.Nodes.ControlPlaneNodes(), &options); err != nil {
		return nil, err
	}

	// On darwin, qemu creates the bridge interface to which the dhcpd server is attached to, so at least one machine has to be created first.
	fmt.Fprintln(options.LogWriter, "creating dhcpd")

	if err = p.CreateDHCPd(ctx, state, request); err != nil {
		return nil, fmt.Errorf("error creating dhcpd: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "creating worker nodes")

	var workerNodeInfo []provision.NodeInfo

	if workerNodeInfo, err = p.createNodes(state, request, request.Nodes.WorkerNodes(), &options); err != nil {
		return nil, err
	}

	var pxeNodeInfo []provision.NodeInfo

	pxeNodes := request.Nodes.PXENodes()
	if len(pxeNodes) > 0 {
		fmt.Fprintln(options.LogWriter, "creating PXE nodes")

		if pxeNodeInfo, err = p.createNodes(state, request, pxeNodes, &options); err != nil {
			return nil, err
		}
	}

	nodeInfo = append(nodeInfo, workerNodeInfo...)

	lbPort := constants.DefaultControlPlanePort

	if len(request.Network.LoadBalancerPorts) > 0 {
		lbPort = request.Network.LoadBalancerPorts[0]
	}

	state.ClusterInfo = provision.ClusterInfo{
		ClusterName: request.Name,
		Network: provision.NetworkInfo{
			Name:              request.Network.Name,
			CIDRs:             request.Network.CIDRs,
			NoMasqueradeCIDRs: request.Network.NoMasqueradeCIDRs,
			GatewayAddrs:      request.Network.GatewayAddrs,
			MTU:               request.Network.MTU,
		},
		Nodes:              nodeInfo,
		ExtraNodes:         pxeNodeInfo,
		KubernetesEndpoint: p.GetExternalKubernetesControlPlaneEndpoint(request.Network, lbPort),
	}

	err = state.Save()
	if err != nil {
		return nil, err
	}

	return state, nil
}
