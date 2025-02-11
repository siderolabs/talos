// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/siderolabs/go-pointer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type EthernetConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *EthernetConfigSuite) TestReconcile() {
	cfg1 := networkcfg.NewEthernetConfigV1Alpha1("enp0s1")
	cfg1.ChannelsConfig = &networkcfg.EthernetChannelsConfig{
		RX: pointer.To[uint32](4),
	}

	ctr, err := container.New(cfg1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)
	suite.Create(cfg)

	ctest.AssertResource(suite, "enp0s1", func(spec *network.EthernetSpec, asrt *assert.Assertions) {
		asrt.Equal(uint32(4), pointer.SafeDeref(spec.TypedSpec().Channels.RX))
	})

	cfg2 := networkcfg.NewEthernetConfigV1Alpha1("enp0s2")
	cfg2.FeaturesConfig = map[string]bool{
		"tx-checksum-ipv4": true,
	}
	cfg2.RingsConfig = &networkcfg.EthernetRingsConfig{
		RX: pointer.To[uint32](16),
	}

	ctr, err = container.New(cfg1, cfg2)
	suite.Require().NoError(err)

	cfgNew := config.NewMachineConfig(ctr)
	cfgNew.Metadata().SetVersion(cfg.Metadata().Version())
	suite.Update(cfgNew)

	ctest.AssertResource(suite, "enp0s1", func(spec *network.EthernetSpec, asrt *assert.Assertions) {
		asrt.Equal(uint32(4), pointer.SafeDeref(spec.TypedSpec().Channels.RX))
	})
	ctest.AssertResource(suite, "enp0s2", func(spec *network.EthernetSpec, asrt *assert.Assertions) {
		asrt.Equal(uint32(16), pointer.SafeDeref(spec.TypedSpec().Rings.RX))
		asrt.Equal(true, spec.TypedSpec().Features["tx-checksum-ipv4"])
	})

	suite.Destroy(cfgNew)

	ctest.AssertNoResource[*network.EthernetSpec](suite, "enp0s1")
	ctest.AssertNoResource[*network.EthernetSpec](suite, "enp0s2")
}

func TestEthernetConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &EthernetConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.EthernetConfigController{}))
			},
		},
	})
}
