// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/talos-systems/talos/pkg/machinery/resources/etcd"
	"github.com/talos-systems/talos/pkg/machinery/resources/k8s"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestSpecSuite(t *testing.T) {
	suite.Run(t, &SpecSuite{
		DefaultSuite: ctest.DefaultSuite{
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

	addresses := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressRoutedID, k8s.NodeAddressFilterNoK8s),
	)

	addresses.TypedSpec().Addresses = []netaddr.IPPrefix{
		netaddr.MustParseIPPrefix("10.0.0.5/24"),
		netaddr.MustParseIPPrefix("192.168.1.1/24"),
		netaddr.MustParseIPPrefix("192.168.1.50/32"),
		netaddr.MustParseIPPrefix("2001:0db8:85a3:0000:0000:8a2e:0370:7334/64"),
		netaddr.MustParseIPPrefix("2002:0db8:85a3:0000:0000:8a2e:0370:7335/64"),
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), addresses))

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
				AdvertisedAddresses: []netaddr.IP{
					netaddr.MustParseIP("10.0.0.5"),
				},
				ListenPeerAddresses: []netaddr.IP{
					netaddr.IPv6Unspecified(),
				},
				ListenClientAddresses: []netaddr.IP{
					netaddr.IPv6Unspecified(),
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
				AdvertisedAddresses: []netaddr.IP{
					netaddr.MustParseIP("192.168.1.1"),
				},
				ListenPeerAddresses: []netaddr.IP{
					netaddr.IPv6Unspecified(),
				},
				ListenClientAddresses: []netaddr.IP{
					netaddr.IPv6Unspecified(),
				},
			},
		},
		{
			name: "only advertised",
			cfg: etcd.ConfigSpec{
				Image: "foo/bar:v1.0.0",
				AdvertiseValidSubnets: []string{
					"192.168.0.0/16",
				},
			},
			expected: etcd.SpecSpec{
				Name:  "worker1",
				Image: "foo/bar:v1.0.0",
				AdvertisedAddresses: []netaddr.IP{
					netaddr.MustParseIP("192.168.1.1"),
					netaddr.MustParseIP("192.168.1.50"),
				},
				ListenPeerAddresses: []netaddr.IP{
					netaddr.IPv6Unspecified(),
				},
				ListenClientAddresses: []netaddr.IP{
					netaddr.IPv6Unspecified(),
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
				AdvertisedAddresses: []netaddr.IP{
					netaddr.MustParseIP("192.168.1.1"),
				},
				ListenPeerAddresses: []netaddr.IP{
					netaddr.IPv6Unspecified(),
				},
				ListenClientAddresses: []netaddr.IP{
					netaddr.IPv6Unspecified(),
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
				AdvertisedAddresses: []netaddr.IP{
					netaddr.MustParseIP("192.168.1.1"),
					netaddr.MustParseIP("192.168.1.50"),
					netaddr.MustParseIP("2001:0db8:85a3:0000:0000:8a2e:0370:7334"),
				},
				ListenPeerAddresses: []netaddr.IP{
					netaddr.MustParseIP("192.168.1.1"),
					netaddr.MustParseIP("192.168.1.50"),
				},
				ListenClientAddresses: []netaddr.IP{
					netaddr.MustParseIP("::1"),
					netaddr.MustParseIP("192.168.1.1"),
					netaddr.MustParseIP("192.168.1.50"),
				},
			},
		},
	} {
		suite.Run(tt.name, func() {
			etcdConfig := etcd.NewConfig(etcd.NamespaceName, etcd.ConfigID)
			*etcdConfig.TypedSpec() = tt.cfg

			suite.Require().NoError(suite.State().Create(suite.Ctx(), etcdConfig))

			suite.AssertWithin(3*time.Second, 100*time.Millisecond, ctest.WrapRetry(func(assert *assert.Assertions, require *require.Assertions) {
				etcdSpec, err := safe.StateGet[*etcd.Spec](suite.Ctx(), suite.State(), etcd.NewSpec(etcd.NamespaceName, etcd.SpecID).Metadata())
				if err != nil {
					assert.NoError(err)

					return
				}

				assert.Equal(tt.expected, *etcdSpec.TypedSpec(), "spec %v", *etcdSpec.TypedSpec())
			}))

			suite.Require().NoError(suite.State().Destroy(suite.Ctx(), etcdConfig.Metadata()))
		})
	}
}
