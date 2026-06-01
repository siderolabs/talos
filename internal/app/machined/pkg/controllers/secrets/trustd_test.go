// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	stdlibx509 "crypto/x509"
	"fmt"
	"net/netip"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestTrustdSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &TrustdSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.TrustdController{}))
			},
		},
	})
}

type TrustdSuite struct {
	ctest.DefaultSuite
}

func (suite *TrustdSuite) TestReconcileControlPlane() {
	rootSecrets := secrets.NewOSRoot(secrets.OSRootID)

	talosCA, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("talos"),
	)
	suite.Require().NoError(err)

	rootSecrets.TypedSpec().IssuingCA = &x509.PEMEncodedCertificateAndKey{
		Crt: talosCA.CrtPEM,
		Key: talosCA.KeyPEM,
	}
	rootSecrets.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{
		{
			Crt: talosCA.CrtPEM,
		},
	}
	rootSecrets.TypedSpec().CertSANDNSNames = []string{"example.com"}
	rootSecrets.TypedSpec().CertSANIPs = []netip.Addr{netip.MustParseAddr("10.4.3.2"), netip.MustParseAddr("10.2.1.3")}
	rootSecrets.TypedSpec().Token = "something"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), rootSecrets))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	networkStatus.TypedSpec().HostnameReady = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), networkStatus))

	certSANs := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
	certSANs.TypedSpec().Append(
		"example.com",
		"foo",
		"foo.example.com",
		"10.2.1.3",
		"10.4.3.2",
		"172.16.0.1",
	)

	certSANs.TypedSpec().FQDN = "foo.example.com"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), certSANs))
	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		certs, err := ctest.Get[*secrets.Trustd](
			suite,
			resource.NewMetadata(
				secrets.NamespaceName,
				secrets.TrustdType,
				secrets.TrustdID,
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		trustdCerts := certs.TypedSpec()

		suite.Assert().Equal(
			[]*x509.PEMEncodedCertificate{
				{
					Crt: talosCA.CrtPEM,
				},
			},
			trustdCerts.AcceptedCAs,
		)

		serverCert, err := trustdCerts.Server.GetCert()
		suite.Require().NoError(err)

		suite.Assert().Equal([]string{"example.com", "foo", "foo.example.com"}, serverCert.DNSNames)
		suite.Assert().Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", serverCert.IPAddresses))

		suite.Assert().Equal("foo.example.com", serverCert.Subject.CommonName)
		suite.Assert().Empty(serverCert.Subject.Organization)

		suite.Assert().Equal(
			stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment,
			serverCert.KeyUsage,
		)
		suite.Assert().Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, serverCert.ExtKeyUsage)

		return nil
	})
}
