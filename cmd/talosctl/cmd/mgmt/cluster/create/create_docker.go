// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create

import (
	"fmt"
	"strings"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/provision"
)

type clusterCreateRequestData struct {
	clusterRequest   provision.ClusterRequest
	provisionOptions []provision.Option
	talosconfig      *clientconfig.Config
}

//nolint:gocyclo,cyclop
func getDockerClusterRequest(
	cOps commonOps,
	dOps dockerOps,
	provisioner provision.Provisioner,
) (clusterCreateRequestData, error) {
	parts := strings.Split(dOps.talosImage, ":")
	cOps.talosVersion = parts[len(parts)-1]

	_, err := config.ParseContractFromVersion(cOps.talosVersion)
	if err != nil {
		currentVersion := helpers.GetTag()
		cOps.talosVersion = currentVersion
		fmt.Printf("failed to derrive Talos version from the docker image, defaulting to %s\n", currentVersion)
	}

	return createClusterRequest(createClusterRequestOps{
		commonOps:   cOps,
		provisioner: provisioner,
		withExtraGenOpts: func(cr provision.ClusterRequest) []generate.Option {
			endpointList := provisioner.GetTalosAPIEndpoints(cr.Network)
			genOptions := []generate.Option{}

			genOptions = append(genOptions, getWithAdditionalSubjectAltNamesGenOps(endpointList)...)
			genOptions = append(genOptions, generate.WithEndpointList(endpointList))

			return genOptions
		},
		withExtraProvisionOpts: func(cr provision.ClusterRequest) []provision.Option {
			provisionOptions := []provision.Option{}
			if dOps.ports != "" {
				portList := strings.Split(dOps.ports, ",")
				provisionOptions = append(provisionOptions, provision.WithDockerPorts(portList))
			}
			provisionOptions = append(provisionOptions, provision.WithDockerPortsHostIP(dOps.hostIP))

			return provisionOptions
		},
		modifyClusterRequest: func(cr provision.ClusterRequest) (provision.ClusterRequest, error) {
			cr.Network.DockerDisableIPv6 = dOps.disableIPv6
			cr.Image = dOps.talosImage

			return cr, nil
		},
		modifyNodes: func(cr provision.ClusterRequest, cp, w []provision.NodeRequest) (controlplanes, workers []provision.NodeRequest, err error) {
			for i := range cp {
				cp[i].Mounts = dOps.mountOpts.Value()
			}
			for i := range w {
				w[i].Mounts = dOps.mountOpts.Value()
			}

			return cp, w, nil
		},
	})
}
