/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package node

import (
	"context"
	"encoding/base64"
	"errors"
	"net"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"

	"github.com/talos-systems/talos/pkg/config/types/v1alpha1/generate"
)

// Request represents the set of options available for configuring a node.
type Request struct {
	Type  generate.Type
	Input generate.Input
	Image string
	Name  string
	IP    net.IP

	// Share of CPUs, in 1e-9 fractions
	NanoCPUs int64
	// Memory limit in bytes
	Memory int64
}

// NewNode creates a node as a container.
func NewNode(clusterName string, req *Request) (err error) {
	data, err := generate.Config(req.Type, &req.Input)
	if err != nil {
		return err
	}

	b64data := base64.StdEncoding.EncodeToString([]byte(data))

	// Create the container config.

	containerConfig := &container.Config{
		Hostname: req.Name,
		Image:    req.Image,
		Env:      []string{"PLATFORM=container", "USERDATA=" + b64data},
		Labels: map[string]string{
			"talos.owned":        "true",
			"talos.cluster.name": clusterName,
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
			NanoCPUs: req.NanoCPUs,
			Memory:   req.Memory,
		},
	}

	// Ensure that the container is created in the talos network.

	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			clusterName: {
				NetworkID: clusterName,
			},
		},
	}

	// Mutate the container configurations based on the node type.

	switch req.Type {
	case generate.TypeInit:
		var osdPort nat.Port
		osdPort, err = nat.NewPort("tcp", "50000")
		if err != nil {
			return err
		}

		var apiServerPort nat.Port
		apiServerPort, err = nat.NewPort("tcp", "443")
		if err != nil {
			return err
		}

		containerConfig.ExposedPorts = nat.PortSet{
			osdPort:       struct{}{},
			apiServerPort: struct{}{},
		}

		hostConfig.PortBindings = nat.PortMap{
			osdPort: []nat.PortBinding{
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
		containerConfig.Volumes["/var/lib/etcd"] = struct{}{}
		if req.IP == nil {
			return errors.New("an IP address must be provided when creating a master node")
		}
	}

	if req.IP != nil {
		networkConfig.EndpointsConfig[clusterName].IPAMConfig = &network.EndpointIPAMConfig{IPv4Address: req.IP.String()}
	}

	// Create the container.

	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		return err
	}

	resp, err := cli.ContainerCreate(ctx, containerConfig, hostConfig, networkConfig, req.Name)
	if err != nil {
		return err
	}

	// Start the container.

	return cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
}
