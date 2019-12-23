// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/pkg/provision"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
	"github.com/talos-systems/talos/pkg/constants"
)

func (p *provisioner) createNodes(ctx context.Context, clusterReq provision.ClusterRequest, input *generate.Input, nodeReqs []provision.NodeRequest) ([]provision.NodeInfo, error) {
	errCh := make(chan error)
	nodeCh := make(chan provision.NodeInfo, len(nodeReqs))

	for _, nodeReq := range nodeReqs {
		go func(nodeReq provision.NodeRequest) {
			nodeInfo, err := p.createNode(ctx, clusterReq, input, nodeReq)
			errCh <- err

			if err == nil {
				nodeCh <- nodeInfo
			}
		}(nodeReq)
	}

	var multiErr *multierror.Error

	for range nodeReqs {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	close(nodeCh)

	nodesInfo := make([]provision.NodeInfo, 0, len(nodeReqs))

	for nodeInfo := range nodeCh {
		nodesInfo = append(nodesInfo, nodeInfo)
	}

	return nodesInfo, multiErr.ErrorOrNil()
}

//nolint: gocyclo
func (p *provisioner) createNode(ctx context.Context, clusterReq provision.ClusterRequest, input *generate.Input, nodeReq provision.NodeRequest) (provision.NodeInfo, error) {
	inputCopy := *input // TOD: this looks like a bug in generate?

	data, err := generate.Config(nodeReq.Type, &inputCopy)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	b64data := base64.StdEncoding.EncodeToString([]byte(data))

	// Create the container config.

	containerConfig := &container.Config{
		Hostname: nodeReq.Name,
		Image:    clusterReq.Image,
		Env:      []string{"PLATFORM=container", "USERDATA=" + b64data},
		Labels: map[string]string{
			"talos.owned":        "true",
			"talos.cluster.name": clusterReq.Name,
			"talos.type":         nodeReq.Type.String(),
		},
		Volumes: map[string]struct{}{
			"/var/lib/containerd": {},
			"/var/lib/kubelet":    {},
			"/etc/cni":            {},
			"/run":                {},
		},
	}

	// Create the host config.

	hostConfig := &container.HostConfig{
		Privileged:  true,
		SecurityOpt: []string{"seccomp:unconfined"},
		Resources: container.Resources{
			NanoCPUs: nodeReq.NanoCPUs,
			Memory:   nodeReq.Memory,
		},
	}

	// Ensure that the container is created in the talos network.

	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			clusterReq.Network.Name: {
				NetworkID: clusterReq.Network.Name,
			},
		},
	}

	// Mutate the container configurations based on the node type.

	switch nodeReq.Type {
	case generate.TypeInit:
		var apidPort nat.Port
		apidPort, err = nat.NewPort("tcp", "50000")

		if err != nil {
			return provision.NodeInfo{}, err
		}

		var apiServerPort nat.Port
		apiServerPort, err = nat.NewPort("tcp", "6443")

		if err != nil {
			return provision.NodeInfo{}, err
		}

		containerConfig.ExposedPorts = nat.PortSet{
			apidPort:      struct{}{},
			apiServerPort: struct{}{},
		}

		hostConfig.PortBindings = nat.PortMap{
			apidPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "50000",
				},
			},
			apiServerPort: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "6443",
				},
			},
		}

		fallthrough
	case generate.TypeControlPlane:
		containerConfig.Volumes[constants.EtcdDataPath] = struct{}{}

		if nodeReq.IP == nil {
			return provision.NodeInfo{}, errors.New("an IP address must be provided when creating a master node")
		}
	}

	if nodeReq.IP != nil {
		networkConfig.EndpointsConfig[clusterReq.Network.Name].IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: nodeReq.IP.String()}
	}

	// Create the container.
	resp, err := p.client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nodeReq.Name)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// Start the container.
	err = p.client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// Inspect the container.
	info, err := p.client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	nodeInfo := provision.NodeInfo{
		ID:   info.ID,
		Name: info.Name,
		Type: nodeReq.Type,

		PrivateIP: net.ParseIP(info.NetworkSettings.Networks[clusterReq.Network.Name].IPAddress),
	}

	return nodeInfo, nil
}

func (p *provisioner) listNodes(ctx context.Context, clusterName string) ([]types.Container, error) {
	filters := filters.NewArgs()
	filters.Add("label", "talos.owned=true")
	filters.Add("label", "talos.cluster.name="+clusterName)

	return p.client.ContainerList(ctx, types.ContainerListOptions{All: true, Filters: filters})
}

func (p *provisioner) destroyNodes(ctx context.Context, clusterName string, options *provision.Options) error {
	containers, err := p.listNodes(ctx, clusterName)
	if err != nil {
		return err
	}

	errCh := make(chan error)

	for _, container := range containers {
		go func(container types.Container) {
			fmt.Fprintln(options.LogWriter, "destroying node", container.Names[0][1:])

			errCh <- p.client.ContainerRemove(ctx, container.ID, types.ContainerRemoveOptions{RemoveVolumes: true, Force: true})
		}(container)
	}

	var multiErr *multierror.Error

	for range containers {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}
