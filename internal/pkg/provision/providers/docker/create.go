// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"encoding/base64"
	"fmt"

	"github.com/talos-systems/talos/cmd/osctl/pkg/client/config"
	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
)

// Create Talos cluster as a set of docker containers on docker network.
//
//nolint: gocyclo
func (p *provisioner) Create(ctx context.Context, request provision.ClusterRequest, opts ...provision.Option) (provision.Cluster, error) {
	options := provision.DefaultOptions()

	for _, opt := range opts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	if err := p.ensureImageExists(ctx, request.Image, &options); err != nil {
		return nil, err
	}

	initNode, err := request.Nodes.FindInitNode()
	if err != nil {
		return nil, err
	}

	fmt.Fprintln(options.LogWriter, "generating PKI and tokens")

	input, err := generate.NewInput(request.Name, fmt.Sprintf("https://%s:6443", initNode.IP), request.KubernetesVersion)
	if err != nil {
		return nil, err
	}

	if options.ForceEndpoint != "" {
		input.AdditionalSubjectAltNames = append(input.AdditionalSubjectAltNames, options.ForceEndpoint)
		input.AdditionalMachineCertSANs = append(input.AdditionalMachineCertSANs, options.ForceEndpoint)
	}

	fmt.Fprintln(options.LogWriter, "creating network", request.Network.Name)

	if err = p.createNetwork(ctx, request.Network); err != nil {
		return nil, fmt.Errorf("a cluster might already exist, run \"osctl cluster destroy\" to permanently delete the existing cluster, and try again: %w", err)
	}

	var nodeInfo []provision.NodeInfo

	fmt.Fprintln(options.LogWriter, "creating master nodes")

	if nodeInfo, err = p.createNodes(ctx, request, input, request.Nodes.MasterNodes()); err != nil {
		return nil, err
	}

	fmt.Fprintln(options.LogWriter, "creating worker nodes")

	var workerNodeInfo []provision.NodeInfo

	if workerNodeInfo, err = p.createNodes(ctx, request, input, request.Nodes.WorkerNodes()); err != nil {
		return nil, err
	}

	nodeInfo = append(nodeInfo, workerNodeInfo...)

	endpoints := []string{"127.0.0.1"}

	if options.ForceEndpoint != "" {
		endpoints = []string{options.ForceEndpoint}
	} else if options.ForceInitNodeAsEndpoint {
		endpoints = []string{initNode.IP.String()}
	}

	res := &result{
		talosConfig: &config.Config{
			Context: request.Name,
			Contexts: map[string]*config.Context{
				request.Name: {
					Endpoints: endpoints,
					CA:        base64.StdEncoding.EncodeToString(input.Certs.OS.Crt),
					Crt:       base64.StdEncoding.EncodeToString(input.Certs.Admin.Crt),
					Key:       base64.StdEncoding.EncodeToString(input.Certs.Admin.Key),
				},
			},
		},

		clusterInfo: provision.ClusterInfo{
			ClusterName: request.Name,
			Network: provision.NetworkInfo{
				Name: request.Network.Name,
				CIDR: request.Network.CIDR,
			},
			Nodes: nodeInfo,
		},
	}

	return res, nil
}
