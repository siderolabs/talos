// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
)

// Create Talos cluster as a set of docker containers on docker network.
func (p *provisioner) Create(ctx context.Context, request provision.ClusterRequest, opts ...provision.Option) (provision.Cluster, error) {
	var err error

	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err = opt(&options); err != nil {
			return nil, err
		}
	}

	statePath := filepath.Join(request.StateDirectory, request.Name)

	fmt.Fprintf(options.LogWriter, "creating state directory in %q\n", statePath)

	if err := os.MkdirAll(statePath, 0o755); err != nil {
		return nil, fmt.Errorf("unable to create state directory: %w", err)
	}

	if err = p.ensureImageExists(ctx, request.Image, &options); err != nil {
		return nil, err
	}

	fmt.Fprintln(options.LogWriter, "creating network", request.Network.Name)

	if err = p.createNetwork(ctx, request.Network); err != nil {
		return nil, fmt.Errorf("unable to create or re-use a docker network: %w", err)
	}

	var nodeInfo []provision.NodeInfo

	fmt.Fprintln(options.LogWriter, "creating controlplane nodes")

	if nodeInfo, err = p.createNodes(ctx, request, request.Nodes.ControlPlaneNodes(), &options, true); err != nil {
		return nil, err
	}

	fmt.Fprintln(options.LogWriter, "creating worker nodes")

	var workerNodeInfo []provision.NodeInfo

	if workerNodeInfo, err = p.createNodes(ctx, request, request.Nodes.WorkerNodes(), &options, false); err != nil {
		return nil, err
	}

	nodeInfo = append(nodeInfo, workerNodeInfo...)

	res := &result{
		clusterInfo: provision.ClusterInfo{
			ClusterName: request.Name,
			Network: provision.NetworkInfo{
				Name:         request.Network.Name,
				CIDRs:        request.Network.CIDRs[:1],
				GatewayAddrs: request.Network.GatewayAddrs[:1],
				MTU:          request.Network.MTU,
			},
			Nodes:              nodeInfo,
			KubernetesEndpoint: p.GetExternalKubernetesControlPlaneEndpoint(request.Network, constants.DefaultControlPlanePort),
		},
		statePath: statePath,
	}

	return res, nil
}
