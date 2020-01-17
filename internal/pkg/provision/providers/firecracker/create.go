// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/internal/pkg/provision"
)

// Create Talos cluster as a set of firecracker micro-VMs.
//
//nolint: gocyclo
func (p *provisioner) Create(ctx context.Context, request provision.ClusterRequest, opts ...provision.Option) (provision.Cluster, error) {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	state := &state{
		ProvisionerName: "firecracker",
		statePath:       filepath.Join(request.StateDirectory, request.Name),
	}

	fmt.Fprintf(options.LogWriter, "creating state directory in %q\n", state.statePath)

	_, err := os.Stat(state.statePath)
	if err == nil {
		return nil, fmt.Errorf(
			"state directory %q already exists, is the cluster %q already running? remove cluster state with osctl cluster destroy",
			state.statePath,
			request.Name,
		)
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("error checking state directory: %w", err)
	}

	if err = os.MkdirAll(state.statePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating state directory: %w", err)
	}

	fmt.Fprintln(options.LogWriter, "creating network", request.Network.Name)

	if err = p.createNetwork(ctx, state, request.Network); err != nil {
		return nil, fmt.Errorf("unable to provision CNI network: %w", err)
	}

	var nodeInfo []provision.NodeInfo

	fmt.Fprintln(options.LogWriter, "creating master nodes")

	if nodeInfo, err = p.createNodes(state, request, request.Nodes.MasterNodes()); err != nil {
		return nil, err
	}

	fmt.Fprintln(options.LogWriter, "creating worker nodes")

	var workerNodeInfo []provision.NodeInfo

	if workerNodeInfo, err = p.createNodes(state, request, request.Nodes.WorkerNodes()); err != nil {
		return nil, err
	}

	nodeInfo = append(nodeInfo, workerNodeInfo...)

	state.ClusterInfo = provision.ClusterInfo{
		ClusterName: request.Name,
		Network: provision.NetworkInfo{
			Name:        request.Network.Name,
			CIDR:        request.Network.CIDR,
			GatewayAddr: request.Network.GatewayAddr,
			MTU:         request.Network.MTU,
		},
		Nodes: nodeInfo,
	}

	// save state
	stateFile, err := os.Create(filepath.Join(state.statePath, stateFileName))
	if err != nil {
		return nil, err
	}

	defer stateFile.Close() //nolint: errcheck

	if err = yaml.NewEncoder(stateFile).Encode(&state); err != nil {
		return nil, fmt.Errorf("error marshaling state: %w", err)
	}

	return state, stateFile.Close()
}
