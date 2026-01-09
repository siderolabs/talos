// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &SpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 3 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&etcdctrl.SpecController{}))
			},
		},
	})
}

type SpecSuite struct {
	ctest.DefaultSuite
}

func (suite *SpecSuite) TestReconcile() {
	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "worker1"
	hostnameStatus.TypedSpec().Domainname = "some.domain"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), hostnameStatus))

	routedAddresses := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s),
	)

	routedAddresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.0.0.5/24"),
		netip.MustParsePrefix("192.168.1.1/24"),
		netip.MustParsePrefix("192.168.1.50/32"),
		netip.MustParsePrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64"),
		netip.MustParsePrefix("2002:0db8:85a3:0000:0000:8a2e:0370:7335/64"),
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), routedAddresses))

	currentAddrs := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressCurrentID, k8s.NodeAddressFilterNoK8s),
	)

	currentAddrs.TypedSpec().Addresses = append(
		[]netip.Prefix{netip.MustParsePrefix("1.3.5.7/32")},
		routedAddresses.TypedSpec().Addresses...,
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), currentAddrs))

	for _, tt := range []struct {
		name     string
		cfg      etcd.ConfigSpec
		expected etcd.SpecSpec
	}{
		{
			name: "defaults",
			cfg: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				ExtraArgs: map[string]string{
					"arg": "value",
				},
			},
			expected: etcd.SpecSpec{
				Name:  "worker1",
				Image: "foo/bar:v1.0.0",
				ExtraArgs: map[string]string{
					"arg": "value",
				},
				AdvertisedAddresses: []netip.Addr{
					netip.MustParseAddr("10.0.0.5"),
				},
				ListenPeerAddresses: []netip.Addr{
					netip.IPv4Unspecified(),
				},
				ListenClientAddresses: []netip.Addr{
					netip.IPv4Unspecified(),
				},
			},
		},
		{
			name: "defaults with exclude",
			cfg: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				AdvertiseExcludeSubnets: []string{
					"10.0.0.5",
				},
			},
			expected: etcd.SpecSpec{
				Name:  "worker1",
				Image: "foo/bar:v1.0.0",
				AdvertisedAddresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
				},
				ListenPeerAddresses: []netip.Addr{
					netip.IPv4Unspecified(),
				},
				ListenClientAddresses: []netip.Addr{
					netip.IPv4Unspecified(),
				},
			},
		},
		{
			name: "only advertised",
			cfg: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				AdvertiseValidSubnets: []string{
					"192.168.0.0/16",
					"1.3.5.7/32",
				},
			},
			expected: etcd.SpecSpec{
				Name:  "worker1",
				Image: "foo/bar:v1.0.0",
				AdvertisedAddresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
					netip.MustParseAddr("192.168.1.50"),
					netip.MustParseAddr("1.3.5.7"),
				},
				ListenPeerAddresses: []netip.Addr{
					netip.IPv4Unspecified(),
				},
				ListenClientAddresses: []netip.Addr{
					netip.IPv4Unspecified(),
				},
			},
		},
		{
			name: "only advertised with exclude",
			cfg: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				AdvertiseValidSubnets: []string{
					"192.168.0.0/16",
				},
				AdvertiseExcludeSubnets: []string{
					"10.0.0.5",
					"192.168.1.50",
				},
			},
			expected: etcd.SpecSpec{
				Name:  "worker1",
				Image: "foo/bar:v1.0.0",
				AdvertisedAddresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
				},
				ListenPeerAddresses: []netip.Addr{
					netip.IPv4Unspecified(),
				},
				ListenClientAddresses: []netip.Addr{
					netip.IPv4Unspecified(),
				},
			},
		},
		{
			name: "advertised and listen",
			cfg: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				AdvertiseValidSubnets: []string{
					"192.168.0.0/16",
					"2001::/16",
				},
				ListenValidSubnets: []string{
					"192.168.0.0/16",
				},
			},
			expected: etcd.SpecSpec{
				Name:  "worker1",
				Image: "foo/bar:v1.0.0",
				AdvertisedAddresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
					netip.MustParseAddr("192.168.1.50"),
					netip.MustParseAddr("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
				},
				ListenPeerAddresses: []netip.Addr{
					netip.MustParseAddr("192.168.1.1"),
					netip.MustParseAddr("192.168.1.50"),
				},
				ListenClientAddresses: []netip.Addr{
					netip.MustParseAddr("127.0.0.1"),
					netip.MustParseAddr("192.168.1.1"),
					netip.MustParseAddr("192.168.1.50"),
				},
			},
		},
	} {
		suite.Run(tt.name, func() {
			etcdConfig := etcd.NewConfig(etcd.NamespaceName, etcd.ConfigID)
			*etcdConfig.TypedSpec() = tt.cfg

			suite.Require().NoError(suite.State().Create(suite.Ctx(), etcdConfig))

			ctest.AssertResource(suite, etcd.SpecID, func(etcdSpec *etcd.Spec, asrt *assert.Assertions) {
				asrt.Equal(tt.expected, *etcdSpec.TypedSpec(), "spec %v", *etcdSpec.TypedSpec())
			})

			suite.Require().NoError(suite.State().Destroy(suite.Ctx(), etcdConfig.Metadata()))
		})
	}
}
