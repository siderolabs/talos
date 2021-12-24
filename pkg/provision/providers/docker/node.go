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
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/provision"
)

type portMap struct {
	exposedPorts nat.PortSet
	portBindings nat.PortMap
}

func (p *provisioner) createNodes(ctx context.Context, clusterReq provision.ClusterRequest, nodeReqs []provision.NodeRequest, options *provision.Options) ([]provision.NodeInfo, error) {
	errCh := make(chan error)
	nodeCh := make(chan provision.NodeInfo, len(nodeReqs))

	for _, nodeReq := range nodeReqs {
		go func(nodeReq provision.NodeRequest) {
			nodeInfo, err := p.createNode(ctx, clusterReq, nodeReq, options)
			if err == nil {
				nodeCh <- nodeInfo
			}

			errCh <- err
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

//nolint:gocyclo
func (p *provisioner) createNode(ctx context.Context, clusterReq provision.ClusterRequest, nodeReq provision.NodeRequest, options *provision.Options) (provision.NodeInfo, error) {
	env := []string{"PLATFORM=container"}

	if !nodeReq.SkipInjectingConfig {
		cfg, err := nodeReq.Config.EncodeString()
		if err != nil {
			return provision.NodeInfo{}, err
		}

		env = append(env, "USERDATA="+base64.StdEncoding.EncodeToString([]byte(cfg)))
	}

	// Create the container config.
	containerConfig := &container.Config{
		Hostname: nodeReq.Name,
		Image:    clusterReq.Image,
		Env:      env,
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
			"/system":             {},
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

	if nodeReq.Type == machine.TypeInit || nodeReq.Type == machine.TypeControlPlane {
		portsToOpen := nodeReq.Ports

		if len(options.DockerPorts) > 0 {
			portsToOpen = append(portsToOpen, options.DockerPorts...)
		}

		generatedPortMap, err := genPortMap(portsToOpen, options.DockerPortsHostIP)
		if err != nil {
			return provision.NodeInfo{}, err
		}

		containerConfig.ExposedPorts = generatedPortMap.exposedPorts

		hostConfig.PortBindings = generatedPortMap.portBindings

		containerConfig.Volumes[constants.EtcdDataPath] = struct{}{}

		if nodeReq.IPs == nil {
			return provision.NodeInfo{}, errors.New("an IP address must be provided when creating a master node")
		}
	}

	if nodeReq.IPs != nil {
		networkConfig.EndpointsConfig[clusterReq.Network.Name].IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: nodeReq.IPs[0].String()}
	}

	// Create the container.
	resp, err := p.client.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, nil, nodeReq.Name)
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

		NanoCPUs: nodeReq.NanoCPUs,
		Memory:   nodeReq.Memory,

		IPs: []net.IP{net.ParseIP(info.NetworkSettings.Networks[clusterReq.Network.Name].IPAddress)},
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

func genPortMap(portList []string, hostIP string) (portMap, error) {
	portSetRet := nat.PortSet{}
	portMapRet := nat.PortMap{}

	for _, port := range portList {
		explodedPortAndProtocol := strings.Split(port, "/")

		if len(explodedPortAndProtocol) != 2 {
			return portMap{}, errors.New("incorrect format for exposed port/protocols")
		}

		explodedPort := strings.Split(explodedPortAndProtocol[0], ":")

		if len(explodedPort) != 2 {
			return portMap{}, errors.New("incorrect format for exposed ports")
		}

		natPort, err := nat.NewPort(explodedPortAndProtocol[1], explodedPort[1])
		if err != nil {
			return portMap{}, err
		}

		portSetRet[natPort] = struct{}{}
		portMapRet[natPort] = []nat.PortBinding{
			{
				HostIP:   hostIP,
				HostPort: explodedPort[0],
			},
		}
	}

	return portMap{portSetRet, portMapRet}, nil
}
