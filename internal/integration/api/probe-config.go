// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
	networkres "github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const (
	googleDNS = "8.8.8.8:53"
	cfDNS     = "1.1.1.1:53"
)

// ProbeConfigSuite tests ProbeConfig functionality via the API.
type ProbeConfigSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName returns the name of the suite.
func (suite *ProbeConfigSuite) SuiteName() string {
	return "api.ProbeConfigSuite"
}

// SetupTest initializes test context.
func (suite *ProbeConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)
}

// TearDownTest cleans up test context.
func (suite *ProbeConfigSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestProbeConfig tests that ProbeConfig documents create ProbeSpec resources.
func (suite *ProbeConfigSuite) TestProbeConfig() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing ProbeConfig on node %q", node)

	// Create a probe that checks if we can reach a public DNS server
	probeConfig := network.NewTCPProbeConfigV1Alpha1("test-probe")
	probeConfig.ProbeInterval = 2 * time.Second
	probeConfig.ProbeFailureThreshold = 3
	probeConfig.TCPEndpoint = googleDNS
	probeConfig.TCPTimeout = 5 * time.Second

	suite.PatchMachineConfig(nodeCtx, probeConfig)

	// Wait for ProbeSpec resource to be created
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, "tcp:"+googleDNS,
		func(spec *networkres.ProbeSpec, asrt *assert.Assertions) {
			asrt.Equal(2*time.Second, spec.TypedSpec().Interval)
			asrt.Equal(3, spec.TypedSpec().FailureThreshold)
			asrt.Equal(googleDNS, spec.TypedSpec().TCP.Endpoint)
			asrt.Equal(5*time.Second, spec.TypedSpec().TCP.Timeout)
			asrt.Equal(networkres.ConfigMachineConfiguration, spec.TypedSpec().ConfigLayer)
		},
	)

	// Update the probe config
	probeConfig.ProbeFailureThreshold = 5
	suite.PatchMachineConfig(nodeCtx, probeConfig)

	// Wait for ProbeSpec resource to be updated
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, "tcp:"+googleDNS,
		func(spec *networkres.ProbeSpec, asrt *assert.Assertions) {
			asrt.Equal(5, spec.TypedSpec().FailureThreshold)
		},
	)

	// Remove the ProbeConfig
	suite.RemoveMachineConfigDocuments(nodeCtx, network.TCPProbeKind)

	// Wait for ProbeSpec resource to be removed
	rtestutils.AssertNoResource[*networkres.ProbeSpec](nodeCtx, suite.T(), suite.Client.COSI, "tcp:"+googleDNS)
}

// TestMultipleProbes tests that multiple ProbeConfig documents create multiple ProbeSpec resources.
func (suite *ProbeConfigSuite) TestMultipleProbes() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing multiple ProbeConfigs on node %q", node)

	// Create first probe
	probeConfig1 := network.NewTCPProbeConfigV1Alpha1("proxy-check")

	probeConfig1.ProbeInterval = 1 * time.Second
	probeConfig1.ProbeFailureThreshold = 3
	probeConfig1.TCPEndpoint = cfDNS
	probeConfig1.TCPTimeout = 10 * time.Second

	// Create second probe
	probeConfig2 := network.NewTCPProbeConfigV1Alpha1("dns-check")
	probeConfig2.ProbeInterval = 5 * time.Second
	probeConfig2.ProbeFailureThreshold = 2
	probeConfig2.TCPEndpoint = googleDNS
	probeConfig2.TCPTimeout = 5 * time.Second

	suite.PatchMachineConfig(nodeCtx, probeConfig1, probeConfig2)

	// Verify both probes are created
	rtestutils.AssertResources(nodeCtx, suite.T(), suite.Client.COSI,
		[]string{"tcp:" + cfDNS, "tcp:" + googleDNS},
		func(spec *networkres.ProbeSpec, asrt *assert.Assertions) {
			switch spec.TypedSpec().TCP.Endpoint {
			case cfDNS:
				asrt.Equal(1*time.Second, spec.TypedSpec().Interval)
				asrt.Equal(3, spec.TypedSpec().FailureThreshold)
			case googleDNS:
				asrt.Equal(5*time.Second, spec.TypedSpec().Interval)
				asrt.Equal(2, spec.TypedSpec().FailureThreshold)
			}

			asrt.Equal(networkres.ConfigMachineConfiguration, spec.TypedSpec().ConfigLayer)
		},
	)

	// Remove all ProbeConfigs
	suite.RemoveMachineConfigDocuments(nodeCtx, network.TCPProbeKind)

	// Verify both probes are removed
	rtestutils.AssertNoResource[*networkres.ProbeSpec](nodeCtx, suite.T(), suite.Client.COSI, "tcp:"+cfDNS)
	rtestutils.AssertNoResource[*networkres.ProbeSpec](nodeCtx, suite.T(), suite.Client.COSI, "tcp:"+googleDNS)
}

// TestProbeStatus tests that ProbeSpec resources create ProbeStatus resources.
func (suite *ProbeConfigSuite) TestProbeStatus() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("testing ProbeStatus on node %q", node)

	// Create a probe with a very short interval
	probeConfig := network.NewTCPProbeConfigV1Alpha1("dns-status-check")
	probeConfig.ProbeInterval = 1 * time.Second
	probeConfig.ProbeFailureThreshold = 1
	probeConfig.TCPEndpoint = googleDNS
	probeConfig.TCPTimeout = 3 * time.Second

	suite.PatchMachineConfig(nodeCtx, probeConfig)

	// Wait for ProbeSpec to be created
	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, "tcp:"+googleDNS,
		func(spec *networkres.ProbeSpec, asrt *assert.Assertions) {
			asrt.Equal(googleDNS, spec.TypedSpec().TCP.Endpoint)
		},
	)

	// Give the probe controller time to run at least one probe
	time.Sleep(3 * time.Second)

	// Verify ProbeStatus is created and has success/failure data
	probeStatuses, err := safe.StateListAll[*networkres.ProbeStatus](nodeCtx, suite.Client.COSI)
	suite.Require().NoError(err)

	var found bool

	for status := range probeStatuses.All() {
		if status.Metadata().ID() == "tcp:"+googleDNS {
			found = true

			suite.T().Logf("ProbeStatus: success=%v, lastError=%s", status.TypedSpec().Success, status.TypedSpec().LastError)
			// The status should have been updated at least once (either success or failure)
			suite.Assert().True(status.TypedSpec().Success || status.TypedSpec().LastError != "")

			break
		}
	}

	suite.Assert().True(found, "expected to find ProbeStatus for tcp:"+googleDNS)

	// Clean up
	suite.RemoveMachineConfigDocuments(nodeCtx, network.TCPProbeKind)
	rtestutils.AssertNoResource[*networkres.ProbeSpec](nodeCtx, suite.T(), suite.Client.COSI, "tcp:"+googleDNS)
}

func init() {
	allSuites = append(allSuites, &ProbeConfigSuite{})
}
