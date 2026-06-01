// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net"
	"net/netip"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/miekg/dns"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
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

func (suite *DNSServer) TestResolvingDo53() {
	suite.testResolving([]network.NameServerSpec{
		{Addr: netip.MustParseAddr("8.8.8.8")},
		{Addr: netip.MustParseAddr("1.1.1.1")},
	})
}

func (suite *DNSServer) TestResolvingDoT() {
	suite.testResolving([]network.NameServerSpec{
		{
			Addr:          netip.MustParseAddr("8.8.8.8"),
			Protocol:      nethelpers.DNSProtocolDNSOverTLS,
			TLSServerName: "dns.google",
		},
		{
			Addr:          netip.MustParseAddr("1.1.1.1"),
			Protocol:      nethelpers.DNSProtocolDNSOverTLS,
			TLSServerName: "cloudflare-dns.com",
		},
	})
}

func (suite *DNSServer) TestResolvingDoH() {
	suite.testResolving([]network.NameServerSpec{
		{
			Addr:          netip.MustParseAddr("1.1.1.1"),
			Protocol:      nethelpers.DNSProtocolDNSOverHTTP,
			TLSServerName: "cloudflare-dns.com",
		},
		{
			Addr:          netip.MustParseAddr("8.8.8.8"),
			Protocol:      nethelpers.DNSProtocolDNSOverHTTP,
			TLSServerName: "dns.google",
		},
	})
}

func (suite *DNSServer) testResolving(nameservers []network.NameServerSpec) {
	port := getDynamicPort(suite.T())

	cfg := network.NewHostDNSConfig(network.HostDNSConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ListenAddresses = makeAddrs(port)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	resolverSpec := network.NewResolverStatus(network.NamespaceName, network.ResolverID)
	resolverSpec.TypedSpec().NameServers = nameservers

	suite.Require().NoError(suite.State().Create(suite.Ctx(), resolverSpec))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(),
		expectedDNSRunners(port),
		func(r *network.DNSResolveCache, assert *assert.Assertions) {
			assert.Equal("running", r.TypedSpec().Status)
		},
	)

	rtestutils.AssertLength[*network.DNSUpstream](suite.Ctx(), suite.T(), suite.State(), len(nameservers))

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
	port := getDynamicPort(suite.T())

	resolverSpec := network.NewResolverStatus(network.NamespaceName, network.ResolverID)
	resolverSpec.TypedSpec().NameServers = xslices.Map(dnsSlice, func(addr string) network.NameServerSpec {
		return network.NameServerSpec{Addr: netip.MustParseAddr(addr)}
	})

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

func (suite *DNSServer) TestResolveMembers() {
	port := getDynamicPort(suite.T())

	const (
		id  = "talos-default-controlplane-1"
		id2 = "foo.example.com."
	)

	member := cluster.NewMember(cluster.NamespaceName, id)
	*member.TypedSpec() = cluster.MemberSpec{
		NodeID: id,
		Addresses: []netip.Addr{
			netip.MustParseAddr("172.20.0.2"),
		},
		Hostname:        id,
		MachineType:     machine.TypeControlPlane,
		OperatingSystem: "Talos dev",
		ControlPlane:    nil,
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), member))

	member = cluster.NewMember(cluster.NamespaceName, id2)
	*member.TypedSpec() = cluster.MemberSpec{
		NodeID: id2,
		Addresses: []netip.Addr{
			netip.MustParseAddr("172.20.0.3"),
		},
		Hostname:        id2,
		MachineType:     machine.TypeWorker,
		OperatingSystem: "Talos dev",
		ControlPlane:    nil,
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), member))

	cfg := network.NewHostDNSConfig(network.HostDNSConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ListenAddresses = makeAddrs(port)
	cfg.TypedSpec().ResolveMemberNames = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(),
		expectedDNSRunners(port),
		func(r *network.DNSResolveCache, assert *assert.Assertions) {
			assert.Equal("running", r.TypedSpec().Status)
		},
	)

	suite.assertResolverResponse(port, id, dns.TypeA, []string{"talos-default-controlplane-1.\t8\tIN\tA\t172.20.0.2"})
	suite.assertResolverResponse(port, id2, dns.TypeA, []string{"foo.example.com.\t8\tIN\tA\t172.20.0.3"})
}

func (suite *DNSServer) assertResolverResponse(port, name string, qType uint16, expectedResponse []string) {
	suite.Assert().EventuallyWithT(
		func(collect *assert.CollectT) {
			asrt := assert.New(collect)

			exchange, err := dns.Exchange(
				&dns.Msg{
					MsgHdr: dns.MsgHdr{Id: dns.Id(), RecursionDesired: true},
					Question: []dns.Question{
						{Name: dns.Fqdn(name), Qtype: qType, Qclass: dns.ClassINET},
					},
				},
				"127.0.0.53:"+port,
			)
			if !asrt.NoError(err) {
				return
			}

			if expectedResponse == nil {
				if !asrt.Equal(dns.RcodeServerFailure, exchange.Rcode) {
					return
				}

				return
			}

			if !asrt.Equal(dns.RcodeSuccess, exchange.Rcode) {
				return
			}

			responses := xslices.Map(exchange.Answer, dns.RR.String)
			asrt.Equal(expectedResponse, responses)
		},
		3*time.Second, 100*time.Millisecond,
	)
}

func (suite *DNSServer) TestResolveStaticHost() {
	port := getDynamicPort(suite.T())

	const (
		id  = "static-host-1"
		id2 = "static-host-2"
	)

	sh1 := network.NewStaticHost(network.NamespaceName, id)
	sh1.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("10.3.5.1"), netip.MustParseAddr("ff00::1")}
	suite.Create(sh1)

	sh2 := network.NewStaticHost(network.NamespaceName, id2)
	sh2.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("10.3.5.2")}
	suite.Create(sh2)

	cfg := network.NewHostDNSConfig(network.HostDNSConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ListenAddresses = makeAddrs(port)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	rtestutils.AssertResources(
		suite.Ctx(), suite.T(), suite.State(),
		expectedDNSRunners(port),
		func(r *network.DNSResolveCache, assert *assert.Assertions) {
			assert.Equal("running", r.TypedSpec().Status)
		},
	)

	suite.assertResolverResponse(port, id, dns.TypeA, []string{"static-host-1.\t8\tIN\tA\t10.3.5.1"})
	suite.assertResolverResponse(port, id, dns.TypeAAAA, []string{"static-host-1.\t8\tIN\tAAAA\tff00::1"})
	suite.assertResolverResponse(port, id2, dns.TypeA, []string{"static-host-2.\t8\tIN\tA\t10.3.5.2"})
	suite.assertResolverResponse(port, id2, dns.TypeAAAA, nil)
}

func TestDNSServer(t *testing.T) {
	t.Parallel()

	suite.Run(t, &DNSServer{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.DNSUpstreamController{}))
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.DNSResolveCacheController{
					Logger: zaptest.NewLogger(t),
					State:  suite.State(),
				}))
			},
		},
	})
}

func getDynamicPort(t *testing.T) string {
	t.Helper()

	l, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := l.Addr().String()

	require.NoError(t, l.Close())

	_, port, err := net.SplitHostPort(addr)
	require.NoError(t, err)

	return port
}

func makeAddrs(port string) []netip.AddrPort {
	return []netip.AddrPort{
		netip.MustParseAddrPort("127.0.0.53:" + port),
		netip.MustParseAddrPort("[::1]:" + port),
	}
}

type DNSUpstreams struct {
	ctest.DefaultSuite
}

func (suite *DNSUpstreams) TestOrder() {
	port := getDynamicPort(suite.T())

	cfg := network.NewHostDNSConfig(network.HostDNSConfigID)
	cfg.TypedSpec().Enabled = true
	cfg.TypedSpec().ListenAddresses = makeAddrs(port)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	resolverSpec := network.NewResolverStatus(network.NamespaceName, network.ResolverID)

	for i, addrs := range [][]string{
		{"1.0.0.1", "8.8.8.8", "1.1.1.1"},
		{"1.1.1.1", "8.8.8.8", "1.0.0.1", "8.0.0.8"},
		{"192.168.0.1"},
	} {
		if !suite.Run(strings.Join(addrs, ","), func() {
			resolverSpec.TypedSpec().NameServers = xslices.Map(addrs, func(addr string) network.NameServerSpec {
				return network.NameServerSpec{Addr: netip.MustParseAddr(addr)}
			})

			switch i {
			case 0:
				suite.Require().NoError(suite.State().Create(suite.Ctx(), resolverSpec))
			default:
				suite.Require().NoError(suite.State().Update(suite.Ctx(), resolverSpec))
			}

			expected := xslices.Map(addrs, func(t string) string { return t + ":53" })

			rtestutils.AssertLength[*network.DNSUpstream](suite.Ctx(), suite.T(), suite.State(), len(addrs))

			var actual []string

			defer func() { suite.Require().Equal(expected, actual) }()

			for suite.Ctx().Err() == nil {
				upstreams, err := safe.ReaderListAll[*network.DNSUpstream](suite.Ctx(), suite.State())
				suite.Require().NoError(err)

				actual = safe.ToSlice(upstreams, func(u *network.DNSUpstream) string { return u.TypedSpec().Value.Conn.Addr() })

				if slices.Equal(expected, actual) {
					break
				}
			}
		}) {
			break
		}
	}
}

func TestDNSUpstreams(t *testing.T) {
	t.Parallel()

	suite.Run(t, &DNSUpstreams{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.DNSUpstreamController{}))
			},
		},
	})
}
