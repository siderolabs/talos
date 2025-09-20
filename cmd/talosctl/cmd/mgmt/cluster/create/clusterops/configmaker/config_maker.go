// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configmaker

import (
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/makers"
)

// DockerOptions are the options for provisioning a docker based Talos cluster.
type DockerOptions makers.MakerOptions[clusterops.Docker]

// GetDockerConfigs returns the cluster configs for docker.
func GetDockerConfigs(options DockerOptions) (clusterops.ClusterConfigs, error) {
	maker, err := makers.NewDocker(options)
	if err != nil {
		return clusterops.ClusterConfigs{}, err
	}

	return maker.GetClusterConfigs()
}

// QemuOptions are the options for provisioning a qemu based Talos cluster.
type QemuOptions makers.MakerOptions[clusterops.Qemu]

// GetQemuConfigs returns the cluster configs for qemu.
func GetQemuConfigs(options QemuOptions) (clusterops.ClusterConfigs, error) {
	maker, err := makers.NewQemu(options)
	if err != nil {
		return clusterops.ClusterConfigs{}, err
	}

	return maker.GetClusterConfigs()
}
