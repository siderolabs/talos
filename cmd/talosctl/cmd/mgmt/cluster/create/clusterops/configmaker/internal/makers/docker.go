// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers

import (
	"net"
	"slices"
	"strings"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/provision"
)

var _ ConfigMaker = &(Docker{})

// Docker is the maker for docker.
type Docker struct {
	*Maker[clusterops.Docker]
}

// NewDocker returns a new initialized Docker Maker.
func NewDocker(ops MakerOptions[clusterops.Docker]) (Docker, error) {
	maker, err := New(ops)
	if err != nil {
		return Docker{}, err
	}

	m := Docker{Maker: &maker}

	m.SetHooks(MakerHooks{
		InitExtra:                func() error { return nil },
		GetExtraConfigBundleOpts: func() error { return nil },
		GetExtraGenOps:           m.getExtraGenOps,
		GetExtraProvisionOpts:    m.getExtraProvisionOpts,
		ModifyClusterRequest:     m.modifyClusterRequest,
		ModifyNodes:              m.modifyNodes,
	})

	if err := m.Init(); err != nil {
		return Docker{}, err
	}

	return m, nil
}

func (m *Docker) getExtraGenOps() error {
	m.GenOps = slices.Concat(m.GenOps, getWithAdditionalSubjectAltNamesGenOps(m.Endpoints))

	return nil
}

func (m *Docker) getExtraProvisionOpts() error {
	if m.EOps.Ports != "" {
		portList := strings.Split(m.EOps.Ports, ",")
		m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{provision.WithDockerPorts(portList)})
	}

	m.ProvisionOps = slices.Concat(m.ProvisionOps, []provision.Option{provision.WithDockerPortsHostIP(m.EOps.HostIP)})

	return nil
}

func (m *Docker) modifyClusterRequest() error {
	m.ClusterRequest.Network.DockerDisableIPv6 = m.EOps.DisableIPv6
	m.ClusterRequest.Image = m.EOps.TalosImage

	return nil
}

func (m *Docker) modifyNodes() error {
	m.ForEachNode(func(i int, node *provision.NodeRequest) {
		node.Mounts = m.EOps.MountOpts.Value()
	})

	return nil
}

func getWithAdditionalSubjectAltNamesGenOps(endpointList []string) []generate.Option {
	return xslices.Map(endpointList, func(endpointHostPort string) generate.Option {
		endpointHost, _, err := net.SplitHostPort(endpointHostPort)
		if err != nil {
			endpointHost = endpointHostPort
		}

		return generate.WithAdditionalSubjectAltNames([]string{endpointHost})
	})
}
