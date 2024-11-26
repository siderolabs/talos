// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package docker

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/netip"
	"runtime"
	"slices"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/go-connections/nat"
	"github.com/hashicorp/go-multierror"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/provision"
)

type portMap struct {
	exposedPorts nat.PortSet
	portBindings nat.PortMap
}

func (p *provisioner) createNodes(
	ctx context.Context,
	clusterReq provision.ClusterRequest,
	nodeReqs []provision.NodeRequest,
	options *provision.Options,
	isControlplane bool,
) ([]provision.NodeInfo, error) {
	errCh := make(chan error)
	nodeCh := make(chan provision.NodeInfo, len(nodeReqs))

	for i, nodeReq := range nodeReqs {
		go func(i int, nodeReq provision.NodeRequest) {
			if i == 0 && isControlplane {
				hostPrefix := ""

				// on Linux, limit listening to localhost, on other OSes Docker engine VM is separate from the host
				if runtime.GOOS == "linux" {
					hostPrefix = "127.0.0.1:"
				}

				nodeReq.Ports = append(
					[]string{
						fmt.Sprintf("%s%d:%d/tcp", hostPrefix, p.mappedTalosAPIPort, constants.ApidPort),
						fmt.Sprintf("%s%d:%d/tcp", hostPrefix, p.mappedKubernetesPort, constants.DefaultControlPlanePort),
					},
					nodeReq.Ports...,
				)
			}

			nodeInfo, err := p.createNode(ctx, clusterReq, nodeReq, options)
			if err == nil {
				nodeCh <- nodeInfo
			}

			errCh <- err
		}(i, nodeReq)
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
	env := []string{
		"PLATFORM=container",
		fmt.Sprintf("TALOSSKU=%dCPU-%dRAM", nodeReq.NanoCPUs/(1000*1000*1000), nodeReq.Memory/(1024*1024)),
	}

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
	}

	// Create the host config.
	mounts := make([]mount.Mount, 0, len(constants.Overlays)+5+len(nodeReq.Mounts))

	for _, path := range []string{"/run", "/system", "/tmp"} {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeTmpfs,
			Target: path,
		})
	}

	for _, path := range append(
		[]string{constants.EphemeralMountPoint, constants.StateMountPoint},
		xslices.Map(constants.Overlays, func(overlay constants.SELinuxLabeledPath) string {
			return overlay.Path
		})...,
	) {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeVolume,
			Target: path,
		})
	}

	mounts = slices.Concat(mounts, nodeReq.Mounts)

	hostConfig := &container.HostConfig{
		Privileged:  true,
		SecurityOpt: []string{"seccomp:unconfined"},
		Resources: container.Resources{
			NanoCPUs: nodeReq.NanoCPUs,
			Memory:   nodeReq.Memory,
		},
		ReadonlyRootfs: true,
		Mounts:         mounts,
	}

	if !clusterReq.Network.DockerDisableIPv6 {
		// enable IPv6
		hostConfig.Sysctls = map[string]string{
			"net.ipv6.conf.all.disable_ipv6": "0",
		}
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

		if nodeReq.IPs == nil {
			return provision.NodeInfo{}, errors.New("an IP address must be provided when creating a controlplane node")
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
	err = p.client.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// Inspect the container.
	info, err := p.client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return provision.NodeInfo{}, err
	}

	// Get the container's IP address.
	var addr netip.Addr

	if network, ok := info.NetworkSettings.Networks[clusterReq.Network.Name]; ok {
		ip := network.IPAddress

		if ip == "" && network.IPAMConfig != nil {
			ip = network.IPAMConfig.IPv4Address
		}

		addr, err = netip.ParseAddr(ip)
		if err != nil {
			return provision.NodeInfo{}, err
		}
	}

	nodeInfo := provision.NodeInfo{
		ID:   info.ID,
		Name: info.Name,
		Type: nodeReq.Type,

		NanoCPUs: nodeReq.NanoCPUs,
		Memory:   nodeReq.Memory,

		IPs: []netip.Addr{addr},
	}

	return nodeInfo, nil
}

func (p *provisioner) listNodes(ctx context.Context, clusterName string) ([]types.Container, error) {
	filters := filters.NewArgs()
	filters.Add("label", "talos.owned=true")
	filters.Add("label", "talos.cluster.name="+clusterName)

	return p.client.ContainerList(ctx, container.ListOptions{All: true, Filters: filters})
}

func (p *provisioner) destroyNodes(ctx context.Context, clusterName string, options *provision.Options) error {
	containers, err := p.listNodes(ctx, clusterName)
	if err != nil {
		return err
	}

	errCh := make(chan error)

	for _, ctr := range containers {
		go func(ctr types.Container) {
			fmt.Fprintln(options.LogWriter, "destroying node", ctr.Names[0][1:])

			errCh <- p.client.ContainerRemove(ctx, ctr.ID, container.RemoveOptions{RemoveVolumes: true, Force: true})
		}(ctr)
	}

	var multiErr *multierror.Error

	for range containers {
		multiErr = multierror.Append(multiErr, <-errCh)
	}

	return multiErr.ErrorOrNil()
}

func genPortMap(portList []string, defaultHostIP string) (portMap, error) {
	portSetRet := nat.PortSet{}
	portMapRet := nat.PortMap{}

	for _, port := range portList {
		portsAndHost, protocol, ok := strings.Cut(port, "/")

		if !ok {
			return portMap{}, errors.New("incorrect format for exposed port/protocols")
		}

		expodedPortsAndHost := strings.Split(portsAndHost, ":")

		var containerPort, hostPort string

		hostIP := defaultHostIP

		switch len(expodedPortsAndHost) {
		case 2:
			hostPort, containerPort = expodedPortsAndHost[0], expodedPortsAndHost[1]
		case 3:
			hostIP, hostPort, containerPort = expodedPortsAndHost[0], expodedPortsAndHost[1], expodedPortsAndHost[2]
		default:
			return portMap{}, errors.New("incorrect format for exposed ports")
		}

		natPort, err := nat.NewPort(protocol, containerPort)
		if err != nil {
			return portMap{}, err
		}

		portSetRet[natPort] = struct{}{}
		portMapRet[natPort] = []nat.PortBinding{
			{
				HostIP:   hostIP,
				HostPort: hostPort,
			},
		}
	}

	return portMap{portSetRet, portMapRet}, nil
}
