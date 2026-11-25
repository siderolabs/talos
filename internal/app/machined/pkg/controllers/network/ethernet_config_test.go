// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type EthernetConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *EthernetConfigSuite) TestReconcile() {
	cfg1 := networkcfg.NewEthernetConfigV1Alpha1("enp0s1")
	cfg1.ChannelsConfig = &networkcfg.EthernetChannelsConfig{
		RX: new(uint32(4)),
	}
	cfg1.FlowControlConfig = &networkcfg.EthernetFlowControlConfig{
		RX: new(bool(true)),
		TX: new(bool(false)),
	}

	ctr, err := container.New(cfg1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	ctest.AssertResource(suite, "enp0s1", func(spec *network.EthernetSpec, asrt *assert.Assertions) {
		asrt.Equal(uint32(4), pointer.SafeDeref(spec.TypedSpec().Channels.RX))
		asrt.True(pointer.SafeDeref(spec.TypedSpec().FlowControl.RX))
		asrt.False(pointer.SafeDeref(spec.TypedSpec().FlowControl.TX))
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))

	cfg2 := networkcfg.NewEthernetConfigV1Alpha1("enp0s2")
	cfg2.FeaturesConfig = map[string]bool{
		"tx-checksum-ipv4": true,
	}
	cfg2.RingsConfig = &networkcfg.EthernetRingsConfig{
		RX: new(uint32(16)),
	}

	ctr, err = container.New(cfg1, cfg2)
	suite.Require().NoError(err)

	cfgNew := config.NewMachineConfig(ctr)
	cfgNew.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(cfgNew)

	ctest.AssertResource(suite, "enp0s1", func(spec *network.EthernetSpec, asrt *assert.Assertions) {
		asrt.Equal(uint32(4), pointer.SafeDeref(spec.TypedSpec().Channels.RX))
		asrt.True(pointer.SafeDeref(spec.TypedSpec().FlowControl.RX))
		asrt.False(pointer.SafeDeref(spec.TypedSpec().FlowControl.TX))
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
	ctest.AssertResource(suite, "enp0s2", func(spec *network.EthernetSpec, asrt *assert.Assertions) {
		asrt.Equal(uint32(16), pointer.SafeDeref(spec.TypedSpec().Rings.RX))
		asrt.Equal(true, spec.TypedSpec().Features["tx-checksum-ipv4"])
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))

	suite.Destroy(cfgNew)

	ctest.AssertNoResource[*network.EthernetSpec](suite, "enp0s1", rtestutils.WithNamespace(network.ConfigNamespaceName))
	ctest.AssertNoResource[*network.EthernetSpec](suite, "enp0s2", rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *EthernetConfigSuite) TestReconcileAlias() {
	status := network.NewLinkStatus(network.NamespaceName, "enp0s3")
	status.TypedSpec().Alias = "net0"
	suite.Create(status)

	ethCfg := networkcfg.NewEthernetConfigV1Alpha1("net0")
	ethCfg.RingsConfig = &networkcfg.EthernetRingsConfig{
		RX: new(uint32(16)),
	}

	ctr, err := container.New(ethCfg)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResource(suite, "net0", func(spec *network.EthernetSpec, asrt *assert.Assertions) {
		asrt.Equal("enp0s3", spec.TypedSpec().LinkName)
		asrt.Equal(uint32(16), pointer.SafeDeref(spec.TypedSpec().Rings.RX))
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func (suite *EthernetConfigSuite) TestReconcileSelector() {
	physical := network.NewLinkStatus(network.NamespaceName, "enp0s4")
	physical.TypedSpec().Type = nethelpers.LinkEther
	physical.TypedSpec().Driver = "e1000"
	suite.Create(physical)

	// matches the selector, but it's not a physical link
	bond := network.NewLinkStatus(network.NamespaceName, "bond0")
	bond.TypedSpec().Type = nethelpers.LinkEther
	bond.TypedSpec().Kind = "bond"
	bond.TypedSpec().Driver = "e1000"
	suite.Create(bond)

	ethCfg := networkcfg.NewEthernetConfigV1Alpha1("e1000-rings")
	ethCfg.Selector.Match = cel.MustExpression(cel.ParseBooleanExpression(`link.driver == "e1000"`, celenv.LinkLocator()))
	ethCfg.RingsConfig = &networkcfg.EthernetRingsConfig{
		RX: new(uint32(16)),
	}

	ctr, err := container.New(ethCfg)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	ctest.AssertResource(suite, "e1000-rings/enp0s4", func(spec *network.EthernetSpec, asrt *assert.Assertions) {
		asrt.Equal("enp0s4", spec.TypedSpec().LinkName)
		asrt.True(spec.TypedSpec().FromSelector)
		asrt.Equal(uint32(16), pointer.SafeDeref(spec.TypedSpec().Rings.RX))
	}, rtestutils.WithNamespace(network.ConfigNamespaceName))
	ctest.AssertNoResource[*network.EthernetSpec](suite, "e1000-rings/bond0", rtestutils.WithNamespace(network.ConfigNamespaceName))
}

func TestEthernetConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &EthernetConfigSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.EthernetConfigController{}))
		},
	})
}
