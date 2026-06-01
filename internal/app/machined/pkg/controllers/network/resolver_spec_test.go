// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net/netip"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	netctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type ResolverSpecSuite struct {
	ctest.DefaultSuite
}

func (suite *ResolverSpecSuite) TestSpec() {
	suite.Require().NoError(suite.Runtime().RegisterController(&netctrl.ResolverSpecController{}))

	spec := network.NewResolverSpec(network.NamespaceName, "resolvers")
	*spec.TypedSpec() = network.ResolverSpecSpec{
		NameServers: []network.NameServerSpec{{Addr: netip.MustParseAddr(constants.DefaultPrimaryResolver)}},
		DNSServers:  []netip.Addr{netip.MustParseAddr(constants.DefaultPrimaryResolver)}, //nolint:staticcheck // backward compatibility
		ConfigLayer: network.ConfigDefault,
	}

	suite.Create(spec)

	ctest.AssertResource(suite, "resolvers", func(r *network.ResolverStatus, asrt *assert.Assertions) {
		asrt.Equal([]netip.Addr{netip.MustParseAddr(constants.DefaultPrimaryResolver)}, r.TypedSpec().DNSServers) //nolint:staticcheck // backward compatibility
		asrt.Equal([]network.NameServerSpec{{Addr: netip.MustParseAddr(constants.DefaultPrimaryResolver)}}, r.TypedSpec().NameServers)
	})
}

func TestResolverSpecSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ResolverSpecSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
