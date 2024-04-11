// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net"
	"net/netip"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/miekg/dns"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type DNSServer struct {
	ctest.DefaultSuite
}

func expectedDNSRunners(port string) []resource.ID {
	return []resource.ID{
		"tcp-127.0.0.53:" + port,
		"udp-127.0.0.53:" + port,
		// our dns server makes no promises about actually starting on IPv6, so we don't check it here either
	}
}

func (suite *DNSServer) TestResolving() {
	dnsSlice := []string{"8.8.8.8", "1.1.1.1"}
	port := must.Value(getDynamicPort())(suite.T())

	cfg := network.NewHostDNSConfig(network.HostDNSConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ListenAddresses = makeAddrs(port)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	resolverSpec := network.NewResolverStatus(network.NamespaceName, network.ResolverID)
	resolverSpec.TypedSpec().DNSServers = xslices.Map(dnsSlice, netip.MustParseAddr)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), resolverSpec))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		expectedDNSRunners(port),
		func(r *network.DNSResolveCache, assert *assert.Assertions) {
			assert.Equal("running", r.TypedSpec().Status)
		},
	)

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

	err := retry.Constant(5*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(func() error {
		r, err := dns.Exchange(msg, "127.0.0.53:"+port)
		if err != nil {
			return retry.ExpectedError(err)
		}

		if r.Rcode != dns.RcodeSuccess {
			return retry.ExpectedErrorf("expected rcode %d, got %d", dns.RcodeSuccess, r.Rcode)
		}

		res = r

		return nil
	})
	suite.Require().NoError(err)
	suite.Require().Equal(dns.RcodeSuccess, res.Rcode, res)
}

func (suite *DNSServer) TestSetupStartStop() {
	dnsSlice := []string{"8.8.8.8", "1.1.1.1"}
	port := must.Value(getDynamicPort())(suite.T())

	resolverSpec := network.NewResolverStatus(network.NamespaceName, network.ResolverID)
	resolverSpec.TypedSpec().DNSServers = xslices.Map(dnsSlice, netip.MustParseAddr)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), resolverSpec))

	cfg := network.NewHostDNSConfig(network.HostDNSConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ListenAddresses = makeAddrs(port)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(),
		expectedDNSRunners(port),
		func(r *network.DNSResolveCache, assert *assert.Assertions) {
			assert.Equal("running", r.TypedSpec().Status)
		})

	rtestutils.AssertLength[*network.DNSUpstream](suite.Ctx(), suite.T(), suite.State(), len(dnsSlice))
	// stop dns resolver

	cfg.TypedSpec().Enabled = false
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	for _, runner := range expectedDNSRunners(port) {
		ctest.AssertNoResource[*network.DNSResolveCache](suite, runner)
	}

	for _, d := range dnsSlice {
		ctest.AssertNoResource[*network.DNSUpstream](suite, d)
	}

	// start dns resolver again
	cfg.TypedSpec().Enabled = true
	suite.Require().NoError(suite.State().Update(suite.Ctx(), cfg))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), expectedDNSRunners(port), func(r *network.DNSResolveCache, assert *assert.Assertions) {
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
					Logger: zaptest.NewLogger(t),
				}))
			},
		},
	})
}

func getDynamicPort() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}

	closeOnce := sync.OnceValue(l.Close)

	defer closeOnce() //nolint:errcheck

	_, port, err := net.SplitHostPort(l.Addr().String())
	if err != nil {
		return "", err
	}

	return port, closeOnce()
}

func makeAddrs(port string) []netip.AddrPort {
	return []netip.AddrPort{
		netip.MustParseAddrPort("127.0.0.53:" + port),
	}
}
