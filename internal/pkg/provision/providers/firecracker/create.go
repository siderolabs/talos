// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package firecracker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/internal/pkg/provision/providers/firecracker/inmemhttp"
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

	state := &state{}

	fmt.Fprintln(options.LogWriter, "creating network", request.Network.Name)

	// build bridge interface name by taking part of checksum of the network name
	// so that interface name is defined by network name, and different networks have
	// different bridge interfaces
	networkNameHash := sha256.Sum256([]byte(request.Network.Name))
	state.bridgeInterfaceName = fmt.Sprintf("%s%s", "talos", hex.EncodeToString(networkNameHash[:])[:8])

	if err := p.createNetwork(ctx, state, request.Network); err != nil {
		return nil, fmt.Errorf("unable to provision CNI network: %w", err)
	}

	httpServer, err := inmemhttp.NewServer(fmt.Sprintf("%s:0", request.Network.GatewayAddr))
	if err != nil {
		return nil, err
	}

	for _, node := range request.Nodes {
		var cfg string

		cfg, err = node.Config.String()
		if err != nil {
			return nil, err
		}

		if err = httpServer.AddFile(fmt.Sprintf("%s.yaml", node.Name), []byte(cfg)); err != nil {
			return nil, err
		}
	}

	state.baseConfigURL = fmt.Sprintf("http://%s/", httpServer.GetAddr())

	httpServer.Serve()
	defer httpServer.Shutdown(ctx) //nolint: errcheck

	state.tempDir, err = ioutil.TempDir("", "talos")
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(options.LogWriter, "created temporary environment in %q\n", state.tempDir)

	var nodeInfo []provision.NodeInfo

	fmt.Fprintln(options.LogWriter, "creating master nodes")

	if nodeInfo, err = p.createNodes(ctx, state, request, request.Nodes.MasterNodes()); err != nil {
		return nil, err
	}

	fmt.Fprintln(options.LogWriter, "creating worker nodes")

	var workerNodeInfo []provision.NodeInfo

	if workerNodeInfo, err = p.createNodes(ctx, state, request, request.Nodes.WorkerNodes()); err != nil {
		return nil, err
	}

	nodeInfo = append(nodeInfo, workerNodeInfo...)

	// TODO: temporary, need to wait for all nodes to finish bootstrapping
	//       before shutting down config HTTP service
	time.Sleep(30 * time.Second)

	state.clusterInfo = provision.ClusterInfo{
		ClusterName: request.Name,
		Network: provision.NetworkInfo{
			Name: request.Network.Name,
			CIDR: request.Network.CIDR,
		},
		Nodes: nodeInfo,
	}

	return state, nil
}
