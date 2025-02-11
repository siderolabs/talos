// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package create //nolint:testpackage

import (
	"bytes"
	"context"
	"fmt"
	"net/netip"
	"testing"

	sideronet "github.com/siderolabs/net"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/mgmt/cluster/create/clustermaker"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/bundle"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/provision"
)

func runCmd(cmd *cobra.Command, args ...string) (*cobra.Command, string, error) { //nolint:unparam
	outBuf := bytes.NewBufferString("")
	cmd.SetOut(outBuf)
	cmd.SetErr(outBuf)
	cmd.SetArgs(args)
	c, err := cmd.ExecuteC()

	return c, outBuf.String(), err
}

func TestCreateCommandInvalidProvisioner(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "--provisioner=asd")
	assert.ErrorContains(t, err, "unsupported provisioner")
}

func TestCreateCommandInvalidProvisionerFlagQemu(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "--provisioner=qemu", "--docker-disable-ipv6=true")
	assert.ErrorContains(t, err, "docker-disable-ipv6 flag has been set but has no effect with the qemu provisioner")
}

func TestCreateCommandInvalidProvisionerFlagDocker(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "--provisioner=docker", "--with-network-chaos=true")
	assert.ErrorContains(t, err, "with-network-chaos flag has been set but has no effect with the docker provisioner")
}

func TestCreateQemuCommandInvalidProvisionerFlag(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "qemu", "--provisioner=docker")
	assert.ErrorContains(t, err, "invalid provisioner")
}

func TestCreateDockerCommandInvalidProvisionerFlag(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "docker", "--provisioner=qemu")
	assert.ErrorContains(t, err, "invalid provisioner")
}

func TestCreateDockerCommandInvalidFlag(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "docker", "--with-network-chaosr=true")
	assert.ErrorContains(t, err, "unknown flag: --with-network-chaosr")
}

func TestCreateQemuCommandInvalidFlag(t *testing.T) {
	_, _, err := runCmd(cluster.Cmd, "create", "qemu", "--docker-disable-ipv6=true")
	assert.ErrorContains(t, err, "unknown flag: --docker-disable-ipv6")
}

func TestCreateDockerCommand(t *testing.T) {
	command, _, _ := runCmd(cluster.Cmd, "create", "docker", "--with-network-chaosr=true") //nolint:errcheck
	assert.Equal(t, "docker", command.Name())
}

func TestCreateQemuCommand(t *testing.T) {
	command, _, _ := runCmd(cluster.Cmd, "create", "qemu", "--docker-disable-ipv6=true") //nolint:errcheck
	assert.Equal(t, "qemu", command.Name())
}

type testClusterMaker struct {
	provisionOpts     []provision.Option
	cfgBundleOpts     []bundle.Option
	genOpts           []generate.Option
	cidr4             netip.Prefix
	versionContract   *config.VersionContract
	inClusterEndpoint string

	partialReq provision.ClusterRequest
	finalReq   provision.ClusterRequest

	postCreateCalled bool
}

func (cm *testClusterMaker) GetPartialClusterRequest() clustermaker.PartialClusterRequest {
	return clustermaker.PartialClusterRequest(cm.partialReq)
}

func (cm *testClusterMaker) AddGenOps(opts ...generate.Option) {
	cm.genOpts = append(cm.genOpts, opts...)
}

func (cm *testClusterMaker) AddProvisionOps(opts ...provision.Option) {
	cm.provisionOpts = append(cm.provisionOpts, opts...)
}

func (cm *testClusterMaker) AddCfgBundleOpts(opts ...bundle.Option) {
	cm.cfgBundleOpts = append(cm.cfgBundleOpts, opts...)
}

func (cm *testClusterMaker) SetInClusterEndpoint(endpoint string) {
	cm.inClusterEndpoint = endpoint
}

func (cm *testClusterMaker) CreateCluster(ctx context.Context, request clustermaker.PartialClusterRequest) error {
	cm.finalReq = provision.ClusterRequest(request)

	return nil
}

func (cm *testClusterMaker) GetCIDR4() netip.Prefix {
	return cm.cidr4
}

func (cm *testClusterMaker) GetVersionContract() *config.VersionContract {
	return cm.versionContract
}

func (cm *testClusterMaker) PostCreate(ctx context.Context) error {
	cm.postCreateCalled = true

	return nil
}

func (cm *testClusterMaker) getProvisionOpts() (*provision.Options, error) {
	options := provision.Options{}

	for _, opt := range cm.provisionOpts {
		if err := opt(&options); err != nil {
			return nil, err
		}
	}

	return &options, nil
}

func (cm *testClusterMaker) getCfgBundleOpts(t *testing.T) bundle.Options {
	options := bundle.Options{}
	for _, opt := range cm.cfgBundleOpts {
		if err := opt(&options); err != nil {
			t.Error("failed to apply option: ", err)
		}
	}

	return options
}

func getTestClustermaker() testClusterMaker {
	cidr4, err := netip.ParsePrefix("10.50.0.0/24")
	if err != nil {
		panic(err)
	}

	totalNodes := 0

	getTestNode := func(isControl bool) provision.NodeRequest {
		ip, err := sideronet.NthIPInNetwork(cidr4, totalNodes+1)
		if err != nil {
			panic(err)
		}

		nodeType := machine.TypeControlPlane
		if !isControl {
			nodeType = machine.TypeWorker
		}

		node := provision.NodeRequest{
			Name: fmt.Sprint("test-node-", totalNodes),
			IPs:  []netip.Addr{ip},
			Type: nodeType,
		}
		totalNodes++

		return node
	}

	return testClusterMaker{
		cidr4: cidr4,
		partialReq: provision.ClusterRequest{
			Name: "test-cluster",
			Network: provision.NetworkRequest{
				Name:  "test-cluster",
				CIDRs: []netip.Prefix{cidr4},
			},
			Nodes: provision.NodeRequests{
				getTestNode(true), getTestNode(true), getTestNode(false), getTestNode(false),
			},
		},
	}
}
