// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"fmt"
	"net/netip"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestAPICertSANsSuite(t *testing.T) {
	suite.Run(t, &APICertSANsSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.APICertSANsController{}))
			},
		},
	})
}

type APICertSANsSuite struct {
	ctest.DefaultSuite
}

func (suite *APICertSANsSuite) TestReconcileControlPlane() {
	rootSecrets := secrets.NewOSRoot(secrets.OSRootID)

	rootSecrets.TypedSpec().CertSANDNSNames = []string{"some.org"}
	rootSecrets.TypedSpec().CertSANIPs = []netip.Addr{netip.MustParseAddr("10.4.3.2"), netip.MustParseAddr("10.2.1.3")}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), rootSecrets))

	hostnameStatus := network.NewHostnameStatus(network.NamespaceName, network.HostnameID)
	hostnameStatus.TypedSpec().Hostname = "bar"
	hostnameStatus.TypedSpec().Domainname = "some.org"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), hostnameStatus))

	nodeAddresses := network.NewNodeAddress(
		network.NamespaceName,
		network.FilteredNodeAddressID(network.NodeAddressAccumulativeID, k8s.NodeAddressFilterNoK8s),
	)
	nodeAddresses.TypedSpec().Addresses = []netip.Prefix{
		netip.MustParsePrefix("10.2.1.3/24"),
		netip.MustParsePrefix("172.16.0.1/32"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), nodeAddresses))

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		certSANs, err := ctest.Get[*secrets.CertSAN](
			suite,
			resource.NewMetadata(
				secrets.NamespaceName,
				secrets.CertSANType,
				secrets.CertSANAPIID,
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		spec := certSANs.TypedSpec()

		suite.Assert().Equal([]string{"bar", "bar.some.org", "some.org"}, spec.DNSNames)
		suite.Assert().Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", spec.IPs))
		suite.Assert().Equal("bar.some.org", spec.FQDN)

		return nil
	})

	ctest.UpdateWithConflicts(suite, rootSecrets, func(rootSecrets *secrets.OSRoot) error {
		rootSecrets.TypedSpec().CertSANDNSNames = []string{"other.org"}

		return nil
	})

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		certSANs, err := ctest.Get[*secrets.CertSAN](
			suite,
			resource.NewMetadata(
				secrets.NamespaceName,
				secrets.CertSANType,
				secrets.CertSANAPIID,
				resource.VersionUndefined,
			),
		)
		if err != nil {
			return err
		}

		spec := certSANs.TypedSpec()

		expectedDNSNames := []string{"bar", "bar.some.org", "other.org"}

		if !slices.Equal(expectedDNSNames, spec.DNSNames) {
			return retry.ExpectedErrorf("expected %v, got %v", expectedDNSNames, spec.DNSNames)
		}

		return nil
	})
}
