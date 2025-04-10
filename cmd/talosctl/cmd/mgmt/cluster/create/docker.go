// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"context"
	"fmt"
	"strings"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clustermaker"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/providers/docker"
)

func createDockerCluster(ctx context.Context, cOps commonOps, dOps dockerOps) error {
	provisioner, err := docker.NewProvisioner(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err := provisioner.Close(); err != nil {
			fmt.Printf("failed to close docker provisioner: %v", err)
		}
	}()

	maker, err := getDockerClusterMaker(cOps, dOps, provisioner)
	if err != nil {
		return err
	}

	return _createDockerCluster(ctx, dOps, maker)
}

func getDockerClusterMaker(cOps commonOps, dOps dockerOps, provisioner provision.Provisioner) (
	clustermaker.ClusterMaker, error,
) {
	talosversion := cOps.TalosVersion
	if talosversion == "" {
		parts := strings.Split(dOps.nodeImage, ":")
		talosversion = parts[len(parts)-1]
	}

	return clustermaker.New(clustermaker.Input{
		Ops:          cOps,
		Provisioner:  provisioner,
		TalosVersion: talosversion,
	})
}

func _createDockerCluster(ctx context.Context, dOps dockerOps, cm clustermaker.ClusterMaker) error {
	clusterReq := cm.GetPartialClusterRequest()
	cm.AddProvisionOps(provision.WithDockerPortsHostIP(dOps.dockerHostIP))

	if dOps.ports != "" {
		portList := strings.Split(dOps.ports, ",")
		cm.AddProvisionOps(provision.WithDockerPorts(portList))
	}

	clusterReq.Image = dOps.nodeImage
	clusterReq.Network.DockerDisableIPv6 = dOps.dockerDisableIPv6

	for i := range clusterReq.Nodes {
		clusterReq.Nodes[i].Mounts = dOps.mountOpts.Value()
	}

	if err := cm.CreateCluster(ctx, clusterReq); err != nil {
		return err
	}

	return cm.PostCreate(ctx)
}
