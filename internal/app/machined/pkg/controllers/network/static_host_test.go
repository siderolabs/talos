// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	cfgconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	networkcfg "github.com/siderolabs/talos/pkg/machinery/config/types/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type StaticHostSuite struct {
	ctest.DefaultSuite
}

func (suite *StaticHostSuite) machineConfigWithStaticHosts(hosts ...*networkcfg.StaticHostConfigV1Alpha1) *config.MachineConfig {
	docs := make([]cfgconfig.Document, 0, len(hosts))
	for _, h := range hosts {
		docs = append(docs, h)
	}

	ctr, err := container.New(docs...)
	suite.Require().NoError(err)

	return config.NewMachineConfig(ctr)
}

func (suite *StaticHostSuite) TestExtraHostEntries() {
	host1 := networkcfg.NewStaticHostConfigV1Alpha1("10.0.0.1")
	host1.Hostnames = []string{"first.example.com", "First"}

	host1v6 := networkcfg.NewStaticHostConfigV1Alpha1("fd00::1")
	host1v6.Hostnames = []string{"first.example.com"}

	host2 := networkcfg.NewStaticHostConfigV1Alpha1("10.0.0.2")
	host2.Hostnames = []string{"second.example.com"}

	suite.Create(suite.machineConfigWithStaticHosts(host1, host1v6, host2))

	ctest.AssertResources(
		suite,
		[]string{"first.example.com", "second.example.com", "first"},
		func(r *network.StaticHost, asrt *assert.Assertions) {
			switch r.Metadata().ID() {
			case "first.example.com":
				asrt.Equal([]netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("fd00::1")}, r.TypedSpec().Addresses)
			case "second.example.com":
				asrt.Equal([]netip.Addr{netip.MustParseAddr("10.0.0.2")}, r.TypedSpec().Addresses)
			case "first":
				asrt.Equal([]netip.Addr{netip.MustParseAddr("10.0.0.1")}, r.TypedSpec().Addresses)
			}
		},
	)
}

func (suite *StaticHostSuite) TestLocalHostname() {
	hostname := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostname.TypedSpec().Hostname = "node-1"
	hostname.TypedSpec().Domainname = "example.com"
	suite.Create(hostname)

	addrs := network.NewNodeAddress(network.NamespaceName, network.NodeAddressCurrentID)
	addrs.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.10/24"),
		netip.MustParsePrefix("fd00::10/64"),
	}
	suite.Create(addrs)

	ctest.AssertResources(
		suite,
		[]string{"node-1", "node-1.example.com"},
		func(r *network.StaticHost, asrt *assert.Assertions) {
			asrt.Equal(
				[]netip.Addr{netip.MustParseAddr("10.0.0.10"), netip.MustParseAddr("fd00::10")},
				r.TypedSpec().Addresses,
			)
		},
	)
}

func (suite *StaticHostSuite) TestEntryRemoval() {
	keep := networkcfg.NewStaticHostConfigV1Alpha1("10.0.0.1")
	keep.Hostnames = []string{"keep.example.com"}

	drop := networkcfg.NewStaticHostConfigV1Alpha1("10.0.0.2")
	drop.Hostnames = []string{"drop.example.com"}

	cfg := suite.machineConfigWithStaticHosts(keep, drop)
	suite.Create(cfg)

	ctest.AssertResources(
		suite,
		[]string{"keep.example.com", "drop.example.com"},
		func(*network.StaticHost, *assert.Assertions) {},
	)

	// drop one entry and re-apply
	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), cfg.Metadata()))
	suite.Create(suite.machineConfigWithStaticHosts(keep))

	ctest.AssertNoResource[*network.StaticHost](suite, "drop.example.com")
	ctest.AssertResource(suite, "keep.example.com", func(r *network.StaticHost, asrt *assert.Assertions) {
		asrt.Equal([]netip.Addr{netip.MustParseAddr("10.0.0.1")}, r.TypedSpec().Addresses)
	}, rtestutils.WithNamespace(network.NamespaceName))
}

func TestStaticHostSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &StaticHostSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.StaticHostController{}))
			},
		},
	})
}
