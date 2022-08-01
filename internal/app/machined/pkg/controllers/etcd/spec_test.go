// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/ctest"
	etcdctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/etcd"
	"github.com/talos-systems/talos/pkg/machinery/resources/etcd"
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
	etcdConfig := etcd.NewConfig(etcd.NamespaceName, etcd.ConfigID)
	*etcdConfig.TypedSpec() = etcd.ConfigSpec{
		ValidSubnets: []string{"0.0.0.0/0", "::/0"},
		Image:        "foo/bar:v1.0.0",
		ExtraArgs: map[string]string{
			"arg": "value",
		},
	}

	suite.Require().NoError(suite.State().Create(suite.Ctx(), etcdConfig))

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "worker1"
	hostnameStatus.TypedSpec().Domainname = "some.domain"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), hostnameStatus))

	suite.AssertWithin(3*time.Second, 100*time.Millisecond, ctest.WrapRetry(func(assert *assert.Assertions, require *require.Assertions) {
		etcdSpec, err := safe.StateGet[*etcd.Spec](suite.Ctx(), suite.State(), etcd.NewSpec(etcd.NamespaceName, etcd.SpecID).Metadata())
		if err != nil {
			assert.NoError(err)

			return
		}

		assert.Equal("foo/bar:v1.0.0", etcdSpec.TypedSpec().Image)
		assert.Equal(map[string]string{"arg": "value"}, etcdSpec.TypedSpec().ExtraArgs)
		assert.NotEqual(netip.Addr{}, etcdSpec.TypedSpec().AdvertisedAddress)
		assert.True(etcdSpec.TypedSpec().ListenAddress.IsUnspecified())
	}))
}
