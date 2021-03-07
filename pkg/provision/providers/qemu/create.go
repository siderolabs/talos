// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/talos-systems/talos/pkg/provision"
	"github.com/talos-systems/talos/pkg/provision/providers/vm"
)

// Create Talos cluster as a set of qemu VMs.
//
//nolint:gocyclo
func (p *provisioner) Create(ctx context.Context, request provision.ClusterRequest, opts ...provision.Option) (provision.Cluster, error) {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	arch := Arch(options.TargetArch)
	if !arch.Valid() {
		return nil, fmt.Errorf("unsupported arch: %q", options.TargetArch)
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

	fmt.Fprintln(options.LogWriter, "creating network", request.Network.Name)

	if err = p.CreateNetwork(ctx, state, request.Network); err != nil {
		return nil, fmt.Errorf("unable to provision CNI network: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "creating load balancer")

	if err = p.CreateLoadBalancer(state, request); err != nil {
		return nil, fmt.Errorf("error creating loadbalancer: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "creating dhcpd")

	if err = p.CreateDHCPd(state, request); err != nil {
		return nil, fmt.Errorf("error creating dhcpd: %w", err)
	}

	var nodeInfo []provision.NodeInfo

	fmt.Fprintln(options.LogWriter, "creating master nodes")

	if nodeInfo, err = p.createNodes(state, request, request.Nodes.MasterNodes(), &options); err != nil {
		return nil, err
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

	state.ClusterInfo = provision.ClusterInfo{
		ClusterName: request.Name,
		Network: provision.NetworkInfo{
			Name:         request.Network.Name,
			CIDRs:        request.Network.CIDRs,
			GatewayAddrs: request.Network.GatewayAddrs,
			MTU:          request.Network.MTU,
		},
		Nodes:      nodeInfo,
		ExtraNodes: pxeNodeInfo,
	}

	err = state.Save()
	if err != nil {
		return nil, err
	}

	return state, nil
}
