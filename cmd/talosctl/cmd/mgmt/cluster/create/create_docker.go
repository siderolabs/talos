// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"fmt"
	"strings"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker"
	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/provision"
)

//nolint:gocyclo,cyclop
func getDockerClusterRequest(cOps clusterops.Common, dOps clusterops.Docker, provisioner provision.Provisioner) (clusterops.ClusterConfigs, error) {
	parts := strings.Split(dOps.TalosImage, ":")
	cOps.TalosVersion = parts[len(parts)-1]

	_, err := config.ParseContractFromVersion(cOps.TalosVersion)
	if err != nil {
		currentVersion := helpers.GetTag()
		fmt.Printf("failed to derrive Talos version from the docker image, defaulting to %s\n", currentVersion)
		cOps.TalosVersion = currentVersion
	}

	return configmaker.GetDockerConfigs(configmaker.DockerOptions{
		ExtraOps:    dOps,
		CommonOps:   cOps,
		Provisioner: provisioner,
	})
}
