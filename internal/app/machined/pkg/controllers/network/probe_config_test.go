// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

const testAPIVersion = "v1alpha1"

type ProbeConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *ProbeConfigSuite) TestNoConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.ProbeConfigController{}))

	// With no config, no ProbeSpec resources should be created
	ctest.AssertNoResource[*network.ProbeSpec](suite, "tcp:proxy.example.com:3128", rtestutils.WithNamespace(network.NamespaceName))
}

func (suite *ProbeConfigSuite) TestSingleProbe() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.ProbeConfigController{}))

	probeConfig := &networkcfg.ProbeConfigV1Alpha1{
		Meta: networkcfg.ProbeConfigV1Alpha1{}.Meta,
	}
	probeConfig.MetaKind = networkcfg.ProbeKind
	probeConfig.MetaAPIVersion = testAPIVersion
	probeConfig.MetaName = "proxy-check"
	probeConfig.ProbeInterval = time.Second
	probeConfig.FailureThreshold = 3
	probeConfig.TCP = &networkcfg.TCPProbeConfigV1Alpha1{
		Endpoint: "proxy.example.com:3128",
		Timeout:  10 * time.Second,
	}

	ctr, err := container.New(probeConfig)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	ctest.AssertResources(
		suite,
		[]string{
			"tcp:proxy.example.com:3128",
		}, func(r *network.ProbeSpec, asrt *assert.Assertions) {
			asrt.Equal(time.Second, r.TypedSpec().Interval)
			asrt.Equal(3, r.TypedSpec().FailureThreshold)
			asrt.Equal("proxy.example.com:3128", r.TypedSpec().TCP.Endpoint)
			asrt.Equal(10*time.Second, r.TypedSpec().TCP.Timeout)
			asrt.Equal(network.ConfigMachineConfiguration, r.TypedSpec().ConfigLayer)
		},
		rtestutils.WithNamespace(network.NamespaceName),
	)

	// Update the probe config
	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		docs := r.Container().Documents()
		probeDoc := docs[0].(*networkcfg.ProbeConfigV1Alpha1)
		probeDoc.FailureThreshold = 5

		return nil
	})

	ctest.AssertResources(
		suite,
		[]string{
			"tcp:proxy.example.com:3128",
		}, func(r *network.ProbeSpec, asrt *assert.Assertions) {
			asrt.Equal(5, r.TypedSpec().FailureThreshold)
		},
		rtestutils.WithNamespace(network.NamespaceName),
	)

	// Remove the config
	suite.Destroy(cfg)

	ctest.AssertNoResource[*network.ProbeSpec](suite, "tcp:proxy.example.com:3128", rtestutils.WithNamespace(network.NamespaceName))
}

func (suite *ProbeConfigSuite) TestMultipleProbes() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.ProbeConfigController{}))

	// Create first probe
	probeConfig1 := &networkcfg.ProbeConfigV1Alpha1{
		Meta: networkcfg.ProbeConfigV1Alpha1{}.Meta,
	}
	probeConfig1.MetaKind = networkcfg.ProbeKind
	probeConfig1.MetaAPIVersion = "v1alpha1"
	probeConfig1.MetaName = "proxy-check"
	probeConfig1.ProbeInterval = time.Second
	probeConfig1.FailureThreshold = 3
	probeConfig1.TCP = &networkcfg.TCPProbeConfigV1Alpha1{
		Endpoint: "proxy.example.com:3128",
		Timeout:  10 * time.Second,
	}

	// Create second probe
	probeConfig2 := &networkcfg.ProbeConfigV1Alpha1{
		Meta: networkcfg.ProbeConfigV1Alpha1{}.Meta,
	}
	probeConfig2.MetaKind = networkcfg.ProbeKind
	probeConfig2.MetaAPIVersion = "v1alpha1"
	probeConfig2.MetaName = "dns-check"
	probeConfig2.ProbeInterval = 5 * time.Second
	probeConfig2.FailureThreshold = 2
	probeConfig2.TCP = &networkcfg.TCPProbeConfigV1Alpha1{
		Endpoint: "8.8.8.8:53",
		Timeout:  5 * time.Second,
	}

	ctr, err := container.New(probeConfig1, probeConfig2)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	// Verify both probes are created
	ctest.AssertResources(
		suite,
		[]string{
			"tcp:proxy.example.com:3128",
		}, func(r *network.ProbeSpec, asrt *assert.Assertions) {
			asrt.Equal("proxy.example.com:3128", r.TypedSpec().TCP.Endpoint)
			asrt.Equal(3, r.TypedSpec().FailureThreshold)
		},
		rtestutils.WithNamespace(network.NamespaceName),
	)

	ctest.AssertResources(
		suite,
		[]string{
			"tcp:8.8.8.8:53",
		}, func(r *network.ProbeSpec, asrt *assert.Assertions) {
			asrt.Equal("8.8.8.8:53", r.TypedSpec().TCP.Endpoint)
			asrt.Equal(2, r.TypedSpec().FailureThreshold)
		},
		rtestutils.WithNamespace(network.NamespaceName),
	)

	suite.Destroy(cfg)

	// Verify both probes are removed
	ctest.AssertNoResource[*network.ProbeSpec](suite, "tcp:proxy.example.com:3128", rtestutils.WithNamespace(network.NamespaceName))
	ctest.AssertNoResource[*network.ProbeSpec](suite, "tcp:8.8.8.8:53", rtestutils.WithNamespace(network.NamespaceName))
}

func TestProbeConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ProbeConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
		},
	})
}
