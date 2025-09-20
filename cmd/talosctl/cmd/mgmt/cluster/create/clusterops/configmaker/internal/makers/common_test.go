// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package makers_test

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clusterops/configmaker/internal/makers"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/provision"
)

type testProvisioner struct {
	provision.Provisioner
}

func (p testProvisioner) GenOptions(r provision.NetworkRequest) []generate.Option {
	return []generate.Option{func(o *generate.Options) error { return nil }}
}

func (p testProvisioner) GetTalosAPIEndpoints(provision.NetworkRequest) []string {
	return []string{"talos-api-endpoint.test"}
}

func (p testProvisioner) GetInClusterKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	return "controlplane-endpoint.test"
}

func (p testProvisioner) GetExternalKubernetesControlPlaneEndpoint(networkReq provision.NetworkRequest, controlPlanePort int) string {
	return "external-kubernetes-controlplane-endpoint.test"
}

type nothingProvider struct{}

func (*nothingProvider) InitExtra() error                { return nil }
func (*nothingProvider) AddExtraGenOps() error           { return nil }
func (*nothingProvider) AddExtraProvisionOpts() error    { return nil }
func (*nothingProvider) AddExtraConfigBundleOpts() error { return nil }
func (*nothingProvider) ModifyClusterRequest() error     { return nil }
func (*nothingProvider) ModifyNodes() error              { return nil }

func getInitializedTestMaker(t *testing.T, cOps clusterops.Common) makers.Maker[any] {
	m, err := makers.New(makers.MakerOptions[any]{CommonOps: cOps, Provisioner: testProvisioner{}})
	require.NoError(t, err)

	m.SetExtraOptionsProvider(&nothingProvider{})

	err = m.Init()
	require.NoError(t, err)

	return m
}

var nodeUUIDHostnameRegex = regexp.MustCompile("^machine-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")

func TestCommonMaker(t *testing.T) {
	cOps := clusterops.GetCommon()
	cOps.Controlplanes = 2
	cOps.Workers = 2
	cOps.NetworkIPv6 = true
	cOps.RootOps.ClusterName = "test-cluster"

	m := getInitializedTestMaker(t, cOps)

	controlplanes := m.ClusterRequest.Nodes.ControlPlaneNodes()
	workers := m.ClusterRequest.Nodes.WorkerNodes()

	assert.Equal(t, 2, len(controlplanes))
	assert.Equal(t, 2, len(workers))

	assert.Equal(t, "test-cluster", m.ClusterRequest.Name)
	assert.Equal(t, "test-cluster", m.ClusterRequest.Network.Name)
	assert.Equal(t, 2, len(m.Cidrs))
	assert.Equal(t, "10.5.0.0/24", m.Cidrs[0].String())
	assert.Equal(t, "fd74:616c:a05::/64", m.Cidrs[1].String())
	assert.Equal(t, []string{"talos-api-endpoint.test"}, m.Endpoints)

	assert.Equal(t, "test-cluster-controlplane-1", controlplanes[0].Name)
	assert.Equal(t, "test-cluster-controlplane-2", controlplanes[1].Name)
	assert.Equal(t, "test-cluster-worker-1", workers[0].Name)
	assert.Equal(t, "test-cluster-worker-2", workers[1].Name)

	for _, node := range append(controlplanes, workers...) {
		assert.Equal(t, 2, len(node.IPs))
	}

	assert.Equal(t, "10.5.0.2", controlplanes[0].IPs[0].String())
	assert.Equal(t, "fd74:616c:a05::2", controlplanes[0].IPs[1].String())
	assert.Equal(t, "10.5.0.3", controlplanes[1].IPs[0].String())
	assert.Equal(t, "fd74:616c:a05::3", controlplanes[1].IPs[1].String())
	assert.Equal(t, "10.5.0.4", workers[0].IPs[0].String())
	assert.Equal(t, "fd74:616c:a05::4", workers[0].IPs[1].String())
	assert.Equal(t, "10.5.0.5", workers[1].IPs[0].String())
	assert.Equal(t, "fd74:616c:a05::5", workers[1].IPs[1].String())

	assert.Equal(t, "controlplane-endpoint.test", m.InClusterEndpoint)

	m.Ops.WithUUIDHostnames = true
	err := m.Init()
	assert.NoError(t, err)

	controlplanes = m.ClusterRequest.Nodes.ControlPlaneNodes()
	workers = m.ClusterRequest.Nodes.WorkerNodes()

	assert.Regexp(t, nodeUUIDHostnameRegex, controlplanes[0].Name)
	assert.Regexp(t, nodeUUIDHostnameRegex, controlplanes[1].Name)
	assert.Regexp(t, nodeUUIDHostnameRegex, workers[0].Name)
	assert.Regexp(t, nodeUUIDHostnameRegex, workers[1].Name)

	_, err = m.GetClusterConfigs()
	assert.NoError(t, err)
}

func TestCommonMaker_MachineConfig(t *testing.T) {
	cOps := clusterops.GetCommon()
	m := getInitializedTestMaker(t, cOps)

	assertConfigDefaultness(t, cOps, m)
}

// assertConfigDefaultness makes sure the maker-generated machine configs are not different from default talos machine configs.
func assertConfigDefaultness[ExtraOps any](t *testing.T, cOps clusterops.Common, m makers.Maker[ExtraOps], desiredExtraGenOps ...generate.Option) {
	var versionContract *config.VersionContract

	secretsBundle, err := secrets.NewBundle(secrets.NewClock(), versionContract)
	require.NoError(t, err)

	// The only allowed differences from the default machine config.
	desiredExtraGenOps = append(desiredExtraGenOps,
		generate.WithSecretsBundle(secretsBundle),
		generate.WithVersionContract(versionContract),
	)

	in, err := generate.NewInput(cOps.RootOps.ClusterName, "controlplane-endpoint.test", cOps.KubernetesVersion,
		desiredExtraGenOps...,
	)
	require.NoError(t, err)

	m.GenOps = append(m.GenOps, generate.WithSecretsBundle(secretsBundle))

	clusterCfgs, err := m.GetClusterConfigs()
	require.NoError(t, err)

	for _, node := range clusterCfgs.ClusterRequest.Nodes {
		assertMachineConfig(t, in, node)
	}
}

func assertMachineConfig(t *testing.T, in *generate.Input, node provision.NodeRequest) {
	cfgExpected, err := in.Config(node.Type)
	require.NoError(t, err)

	cfgGot := node.Config

	cfgGot = cfgGot.RedactSecrets("secret")
	cfgExpected = cfgExpected.RedactSecrets("secret")

	cfgExpectedBytes, err := cfgExpected.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)
	cfgGotBytes, err := cfgGot.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	require.NoError(t, err)

	assert.Equal(t, string(cfgExpectedBytes), string(cfgGotBytes))
}
