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
	"github.com/docker/docker/api/types/strslice"
	"github.com/docker/docker/client"
	"github.com/talos-systems/talos/pkg/userdata/generate"
)

// Request represents the set of options available for configuring a node.
type Request struct {
	Type  generate.Type
	Input *generate.Input
	Image string
	Name  string
	IP    net.IP
}

// NewNode creates a node as a container.
func NewNode(clusterName string, req *Request) (err error) {
	// Generate the userdata for the node.

	data, err := generate.Userdata(req.Type, req.Input)
	if err != nil {
		return err
	}

	b64data := base64.StdEncoding.EncodeToString([]byte(data))

	// Create the container config.

	containerConfig := &container.Config{
		Hostname:   req.Name,
		Image:      req.Image,
		Entrypoint: strslice.StrSlice{"/init"},
		Cmd:        strslice.StrSlice{"--in-container", "--userdata=" + b64data},
		Labels: map[string]string{
			"talos.owned":        "true",
			"talos.cluster.name": clusterName,
		},
		Volumes: map[string]struct{}{
			"/var/lib/containerd": {},
			"/var/lib/kubelet":    {},
			"/etc/cni":            {},
		},
	}

	// Create the host config.

	hostConfig := &container.HostConfig{
		Privileged:  true,
		SecurityOpt: []string{"seccomp:unconfined"},
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
