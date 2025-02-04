// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/siderolabs/gen/xslices"

	clusterpkg "github.com/siderolabs/talos/pkg/cluster"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/docker"
)

func createDockerCluster(ctx context.Context, cOps CommonOps, dOps dockerOps) error {
	provisioner, err := docker.NewDockerProvisioner(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err := provisioner.Close(); err != nil {
			fmt.Printf("failed to close docker provisioner: %v", err)
		}
	}()

	getTalosVersionDocker := func() string {
		if cOps.TalosVersion != "" {
			return cOps.TalosVersion
		}

		parts := strings.Split(dOps.nodeImage, ":")

		return parts[len(parts)-1]
	}

	portList := []string{}
	if dOps.ports != "" {
		portList = strings.Split(dOps.ports, ",")
	}

	getAdditionalOptions := func(cOps CommonOps, base clusterCreateBase) (additional additionalOptions, err error) {
		additional.provisionOpts = append(additional.provisionOpts, provision.WithDockerPortsHostIP(dOps.dockerHostIP))
		if len(portList) != 0 {
			additional.provisionOpts = append(additional.provisionOpts, provision.WithDockerPorts(portList))
		}

		return additional, nil
	}

	base, err := getBase(cOps, provisioner, getTalosVersionDocker, getAdditionalOptions)
	if err != nil {
		return err
	}

	request := docker.ClusterRequest{
		ClusterRequestBase: base.clusterRequest,
		Image:              dOps.nodeImage,
		Network: docker.NetworkRequest{
			NetworkRequestBase: base.clusterRequest.Network,
			DockerDisableIPv6:  dOps.dockerDisableIPv6,
		},
		Nodes: docker.NodeRequests{},
	}

	baseNodes := slices.Concat(base.clusterRequest.Controlplanes, base.clusterRequest.Workers)
	for _, n := range baseNodes {
		node := docker.NodeRequest{
			NodeRequestBase: n,
			Mounts:          dOps.mountOpts.Value(),
			Ports:           portList,
		}

		request.Nodes = append(request.Nodes, node)
	}

	cluster, err := provisioner.Create(ctx, request, base.provisionOptions...)
	if err != nil {
		return err
	}

	nodeApplyCfgs := xslices.Map(request.Nodes, func(n docker.NodeRequest) clusterpkg.NodeApplyConfig {
		return clusterpkg.NodeApplyConfig{NodeAddress: clusterpkg.NodeAddress{IP: n.IPs[0]}, Config: n.Config}
	})

	return postCreate(ctx, cOps, base.bundleTalosconfig, cluster, base.provisionOptions, nodeApplyCfgs)
}
