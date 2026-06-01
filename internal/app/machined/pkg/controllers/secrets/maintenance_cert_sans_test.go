// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestMaintenanceCertSANsSuite(t *testing.T) {
	suite.Run(t, &MaintenanceCertSANsSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 2 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.MaintenanceCertSANsController{}))
			},
		},
	})
}

type MaintenanceCertSANsSuite struct {
	ctest.DefaultSuite
}

func (suite *MaintenanceCertSANsSuite) TestReconcile() {
	nodeAddresses := network.NewNodeAddress(
		network.NamespaceName,
		network.NodeAddressAccumulativeID,
	)
	nodeAddresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.2.1.3/24"),
		netip.MustParsePrefix("172.16.0.1/32"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodeAddresses))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.CertSANMaintenanceID},
		func(certSANs *secrets.CertSAN, asrt *assert.Assertions) {
			asrt.Empty(certSANs.TypedSpec().DNSNames)
			asrt.Equal("[10.2.1.3 127.0.0.1 172.16.0.1 ::1]", fmt.Sprintf("%v", certSANs.TypedSpec().IPs))
			asrt.Equal(constants.MaintenanceServiceCommonName, certSANs.TypedSpec().FQDN)
		})

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "bar"
	hostnameStatus.TypedSpec().Domainname = "some.org"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), hostnameStatus))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.CertSANMaintenanceID},
		func(certSANs *secrets.CertSAN, asrt *assert.Assertions) {
			asrt.Equal([]string{"bar", "bar.some.org"}, certSANs.TypedSpec().DNSNames)
		})
}
