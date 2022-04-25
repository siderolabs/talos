// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package podman

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/containers/common/libnetwork/types"
	"github.com/containers/podman/v4/libpod/define"
	"github.com/containers/podman/v4/pkg/bindings/containers"
	"github.com/containers/podman/v4/pkg/domain/entities"
	"github.com/containers/podman/v4/pkg/specgen"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/provision"
)

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
	env := map[string]string{"PLATFORM": "container"}

	if !nodeReq.SkipInjectingConfig {
		cfg, err := nodeReq.Config.EncodeString()
		if err != nil {
			return provision.NodeInfo{}, err
		}

		env["USERDATA"] = base64.StdEncoding.EncodeToString([]byte(cfg))
	}

	// Create the container config.
	spec := specgen.NewSpecGenerator(clusterReq.Image, false)

	spec.Name = nodeReq.Name
	spec.Hostname = nodeReq.Name
	spec.Env = env
	spec.Labels = map[string]string{
		"talos.owned":        "true",
		"talos.cluster.name": clusterReq.Name,
		"talos.type":         nodeReq.Type.String(),
	}
	spec.Volumes = []*specgen.NamedVolume{
		{Dest: "/var/lib/containerd"},
		{Dest: "/var/lib/kubelet"},
		{Dest: "/etc/cni"},
		{Dest: "/run"},
		{Dest: "/system"},
	}
	spec.Privileged = true

	// Set the container network configurations based on the node type.
	if nodeReq.Type == machine.TypeInit || nodeReq.Type == machine.TypeControlPlane {
		ports, mappings, err := genPortMapping(nodeReq.Ports)
		if err != nil {
			return provision.NodeInfo{}, err
		}

		if nodeReq.IPs == nil {
			return provision.NodeInfo{}, errors.New("an IP address must be provided when creating a master node")
		}

		spec.PortMappings = mappings
		spec.Expose = ports
		spec.Networks = map[string]types.PerNetworkOptions{
			clusterReq.Network.Name: {
				StaticIPs: nodeReq.IPs,
			},
		}
	} else {
		spec.Networks = map[string]types.PerNetworkOptions{
			clusterReq.Name: {},
		}
	}

	// Create the container.
	resp, err := containers.CreateWithSpec(p.connection, spec, &containers.CreateOptions{})
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// Start the container.
	err = containers.Start(p.connection, resp.ID, &containers.StartOptions{})
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// Wait for the container to have started.
	_, err = containers.Wait(p.connection, resp.ID, &containers.WaitOptions{
		Condition: []define.ContainerStatus{define.ContainerStateRunning},
	})
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// Inspect the container.
	info, err := containers.Inspect(p.connection, resp.ID, &containers.InspectOptions{})
	if err != nil {
		return provision.NodeInfo{}, err
	}

	nodeInfo := provision.NodeInfo{
		ID:   info.ID,
		Name: info.Name,
		Type: nodeReq.Type,

		IPs: []net.IP{net.ParseIP(info.NetworkSettings.Networks[clusterReq.Network.Name].IPAddress)},
	}

	return nodeInfo, nil
}

func (p *provisioner) listNodes(ctx context.Context, clusterName string) ([]entities.ListContainer, error) {
	filters := map[string][]string{
		"label": {"talos.owned=true", "talos.cluster.name=" + clusterName},
	}

	options := containers.ListOptions{Filters: filters}

	return containers.List(p.connection, &options)
}

func (p *provisioner) destroyNodes(ctx context.Context, clusterName string, options *provision.Options) error {
	boxes, err := p.listNodes(p.connection, clusterName)
	if err != nil {
		return err
	}

	errCh := make(chan error)

	for _, container := range boxes {
		go func(container entities.ListContainer) {
			fmt.Fprintln(options.LogWriter, "destroying node", container.Names[0][1:])

			_, err = containers.Remove(p.connection, container.ID, &containers.RemoveOptions{
				Volumes: &[]bool{true}[0],
				Force:   &[]bool{true}[0],
			})
			errCh <- err
		}(container)
	}

	var multiErr *multierror.Error

	for range boxes {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}

func genPortMapping(portList []string) (map[uint16]string, []types.PortMapping, error) {
	ports := map[uint16]string{}
	mappings := []types.PortMapping{}

	for _, port := range portList {
		explodedPortAndProtocol := strings.Split(port, "/")

		if len(explodedPortAndProtocol) != 2 {
			return ports, mappings, errors.New("incorrect format for exposed port/protocols")
		}

		explodedPort := strings.Split(explodedPortAndProtocol[0], ":")

		if len(explodedPort) != 2 {
			return ports, mappings, errors.New("incorrect format for exposed ports")
		}

		port, err := strconv.Atoi(explodedPort[1])
		if err != nil {
			return nil, nil, err
		}

		ports[uint16(port)] = ""

		mappings = append(mappings, types.PortMapping{
			HostIP:        "0.0.0.0",
			ContainerPort: uint16(port),
			HostPort:      uint16(port),
		})
	}

	return ports, mappings, nil
}
