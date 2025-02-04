// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

// Create Talos cluster as a set of qemu VMs.
//
//nolint:gocyclo,cyclop
func (p *Provisioner) Create(ctx context.Context, request vm.ClusterRequest, opts ...provision.Option) (provision.Cluster, error) {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	kvmErr := checkKVM()
	if kvmErr != nil {
		fmt.Println(kvmErr)
		fmt.Println("running without KVM")

		options.UseKvm = false
	} else {
		options.UseKvm = true
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

	controlplanes := xslices.Filter(request.Nodes, func(n vm.NodeRequest) bool { return n.Type.IsControlPlane() })
	controlplaneIps := xslices.Map(controlplanes, func(n vm.NodeRequest) string { return n.IPs[0].String() })

	if err = p.CreateLoadBalancer(state, request, controlplaneIps); err != nil {
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

	fmt.Fprintln(options.LogWriter, "creating dhcpd")

	if err = p.CreateDHCPd(state, request); err != nil {
		return nil, fmt.Errorf("error creating dhcpd: %w", err)
	}

	var nodeInfo []provision.NodeInfo

	fmt.Fprintln(options.LogWriter, "creating controlplane nodes")

	if nodeInfo, err = p.createNodes(state, request, controlplanes, &options); err != nil {
		return nil, err
	}

	fmt.Fprintln(options.LogWriter, "creating worker nodes")

	var workerNodeInfo []provision.NodeInfo

	workers := xslices.Filter(request.Nodes, func(n vm.NodeRequest) bool { return !n.Type.IsControlPlane() })
	if workerNodeInfo, err = p.createNodes(state, request, workers, &options); err != nil {
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
			Name:         request.Network.Name,
			CIDRs:        request.Network.CIDRs,
			GatewayAddrs: request.Network.GatewayAddrs,
			MTU:          request.Network.MTU,
		},
		Nodes:              nodeInfo,
		ExtraNodes:         pxeNodeInfo,
		KubernetesEndpoint: p.GetExternalKubernetesControlPlaneEndpoint(request.Network.NetworkRequestBase, lbPort),
	}

	err = state.Save()
	if err != nil {
		return nil, err
	}

	return state, nil
}

func checkKVM() error {
	f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("error opening /dev/kvm, please make sure KVM support is enabled in Linux kernel: %w", err)
	}

	return f.Close()
}
