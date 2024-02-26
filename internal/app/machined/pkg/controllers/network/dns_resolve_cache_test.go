// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/miekg/dns"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type DNSServer struct {
	ctest.DefaultSuite
}

func (suite *DNSServer) TestResolving() {
	dnsSlice := []string{"8.8.8.8", "1.1.1.1"}

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						LocalDNS: pointer.To(true),
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	resolverSpec := network.NewResolverStatus(network.NamespaceName, network.ResolverID)
	resolverSpec.TypedSpec().DNSServers = xslices.Map(dnsSlice, netip.MustParseAddr)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), resolverSpec))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{"tcp", "udp"}, func(r *network.DNSResolveCache, assert *assert.Assertions) {
		assert.Equal("running", r.TypedSpec().Status)
	})

	rtestutils.AssertLength[*network.DNSUpstream](suite.Ctx(), suite.T(), suite.State(), len(dnsSlice))

	msg := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:               dns.Id(),
			RecursionDesired: true,
		},
		Question: []dns.Question{
			{
				Name:   dns.Fqdn("google.com"),
				Qtype:  dns.TypeA,
				Qclass: dns.ClassINET,
			},
		},
	}

	var res *dns.Msg

	err := retry.Constant(2*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		r, err := dns.Exchange(msg, "127.0.0.53:10700")

		res = r

		return retry.ExpectedError(err)
	})
	suite.Require().NoError(err)
	suite.Require().Equal(dns.RcodeSuccess, res.Rcode, res)
}

func (suite *DNSServer) TestSetupStartStop() {
	dnsSlice := []string{"8.8.8.8", "1.1.1.1"}

	resolverSpec := network.NewResolverStatus(network.NamespaceName, network.ResolverID)
	resolverSpec.TypedSpec().DNSServers = xslices.Map(dnsSlice, netip.MustParseAddr)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), resolverSpec))

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						LocalDNS: pointer.To(true),
					},
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{"tcp", "udp"}, func(r *network.DNSResolveCache, assert *assert.Assertions) {
		assert.Equal("running", r.TypedSpec().Status)
	})

	rtestutils.AssertLength[*network.DNSUpstream](suite.Ctx(), suite.T(), suite.State(), len(dnsSlice))
	// stop dns resolver

	cfg.Container().RawV1Alpha1().MachineConfig.MachineFeatures.LocalDNS = pointer.To(false)

	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	ctest.AssertNoResource[*network.DNSResolveCache](suite, "tcp")
	ctest.AssertNoResource[*network.DNSResolveCache](suite, "udp")

	for _, d := range dnsSlice {
		ctest.AssertNoResource[*network.DNSUpstream](suite, d)
	}

	// start dns resolver again

	cfg.Container().RawV1Alpha1().MachineConfig.MachineFeatures.LocalDNS = pointer.To(true)

	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{"tcp", "udp"}, func(r *network.DNSResolveCache, assert *assert.Assertions) {
		assert.Equal("running", r.TypedSpec().Status)
	})

	rtestutils.AssertLength[*network.DNSUpstream](suite.Ctx(), suite.T(), suite.State(), len(dnsSlice))
}

func TestDNSServer(t *testing.T) {
	suite.Run(t, &DNSServer{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.DNSUpstreamController{}))
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.DNSResolveCacheController{
					Addr:   "127.0.0.53:10700",
					AddrV6: "[::1]:10700",
					Logger: zaptest.NewLogger(t),
				}))
			},
		},
	})
}
