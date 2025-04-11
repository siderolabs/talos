// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"context"
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
	"github.com/siderolabs/talos/pkg/grpc/gen"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

func TestAPISuite(t *testing.T) {
	ca, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("talos"),
	)
	if err != nil {
		t.Errorf("failed to create certificate authority: %v", err)
	}

	suite.Run(t, &APISuite{
		ca: *ca,
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.APIController{
					NewRemoteGenerator: func(token string, endpoints []string, acceptedCAs []*x509.PEMEncodedCertificate) (secretsctrl.CertificateGenerator, error) {
						g, err := gen.NewLocalGenerator(ca.KeyPEM, ca.CrtPEM)
						if err != nil {
							return nil, err
						}
						return mockGenerator{g: g}, nil
					},
				}))
			},
		},
	})
}

type mockGenerator struct {
	g *gen.LocalGenerator
}

func (t mockGenerator) IdentityContext(_ context.Context, csr *x509.CertificateSigningRequest) (ca, crt []byte, err error) {
	return t.g.Identity(csr)
}

func (t mockGenerator) Close() error {
	return nil
}

type APISuite struct {
	ctest.DefaultSuite
	ca x509.CertificateAuthority
}

func (suite *APISuite) TestReconcileControlPlane() {
	rootSecrets := secrets.NewOSRoot(secrets.OSRootID)

	rootSecrets.TypedSpec().IssuingCA = &x509.PEMEncodedCertificateAndKey{
		Crt: suite.ca.CrtPEM,
		Key: suite.ca.KeyPEM,
	}
	rootSecrets.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{
		{
			Crt: suite.ca.CrtPEM,
		},
	}
	rootSecrets.TypedSpec().CertSANDNSNames = []string{"example.com"}
	rootSecrets.TypedSpec().CertSANIPs = []netip.Addr{netip.MustParseAddr("10.4.3.2"), netip.MustParseAddr("10.2.1.3")}
	rootSecrets.TypedSpec().Token = "token-foo"
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
		certs, err := ctest.Get[*secrets.API](
			suite,
			resource.NewMetadata(
				secrets.NamespaceName,
				secrets.APIType,
				secrets.APIID,
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		apiCerts := certs.TypedSpec()

		suite.Assert().Equal(
			[]*x509.PEMEncodedCertificate{
				{
					Crt: suite.ca.CrtPEM,
				},
			},
			apiCerts.AcceptedCAs,
		)

		serverCert, err := apiCerts.Server.GetCert()
		suite.Require().NoError(err)

		suite.Assert().Equal([]string{"example.com", "foo", "foo.example.com"}, serverCert.DNSNames)
		suite.Assert().Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", serverCert.IPAddresses))

		suite.Assert().Equal("foo.example.com", serverCert.Subject.CommonName)
		suite.Assert().Empty(serverCert.Subject.Organization)

		suite.Assert().Equal(
			stdlibx509.KeyUsageDigitalSignature,
			serverCert.KeyUsage,
		)
		suite.Assert().Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, serverCert.ExtKeyUsage)

		clientCert, err := apiCerts.Client.GetCert()
		suite.Require().NoError(err)

		suite.Assert().Empty(clientCert.DNSNames)
		suite.Assert().Empty(clientCert.IPAddresses)

		suite.Assert().Equal("foo.example.com", clientCert.Subject.CommonName)
		suite.Assert().Equal([]string{string(role.Impersonator)}, clientCert.Subject.Organization)

		suite.Assert().Equal(
			stdlibx509.KeyUsageDigitalSignature,
			clientCert.KeyUsage,
		)
		suite.Assert().Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageClientAuth}, clientCert.ExtKeyUsage)

		return nil
	})
}

func (suite *APISuite) TestReconcileWorker() {
	rootSecrets := secrets.NewOSRoot(secrets.OSRootID)

	rootSecrets.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{
		{
			Crt: suite.ca.CrtPEM,
		},
	}

	rootSecrets.TypedSpec().CertSANDNSNames = []string{"example.com"}
	rootSecrets.TypedSpec().CertSANIPs = []netip.Addr{netip.MustParseAddr("10.4.3.2"), netip.MustParseAddr("10.2.1.3")}
	rootSecrets.TypedSpec().Token = "token-bar"
	rootSecrets.TypedSpec().Algorithm = int(algorithm)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), rootSecrets))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Require().NoError(suite.State().Create(suite.Ctx(), machineType))

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	networkStatus.TypedSpec().HostnameReady = true
	suite.Require().NoError(suite.State().Create(suite.Ctx(), networkStatus))

	endpoints := k8s.NewEndpoint(k8s.ControlPlaneNamespaceName, "1")
	endpoints.TypedSpec().Addresses = []netip.Addr{
		netip.MustParseAddr("172.20.0.2"),
		netip.MustParseAddr("172.20.0.3"),
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), endpoints))

	certSANs := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANAPIID)
	certSANs.TypedSpec().Append(
		"example.com",
		"bar",
		"bar.example.com",
		"10.2.1.3",
		"10.4.3.2",
		"172.16.0.1",
	)

	certSANs.TypedSpec().FQDN = "bar.example.com"

	suite.Require().NoError(suite.State().Create(suite.Ctx(), certSANs))

	suite.AssertWithin(10*time.Second, 100*time.Millisecond, func() error {
		certs, err := ctest.Get[*secrets.API](
			suite,
			resource.NewMetadata(
				secrets.NamespaceName,
				secrets.APIType,
				secrets.APIID,
				resource.VersionUndefined,
			),
		)
		if err != nil {
			if state.IsNotFoundError(err) {
				return retry.ExpectedError(err)
			}

			return err
		}

		apiCerts := certs.TypedSpec()

		suite.Assert().Equal(
			[]*x509.PEMEncodedCertificate{
				{
					Crt: suite.ca.CrtPEM,
				},
			},
			apiCerts.AcceptedCAs,
		)

		serverCert, err := apiCerts.Server.GetCert()
		suite.Require().NoError(err)

		suite.Assert().Equal([]string{"example.com", "bar", "bar.example.com"}, serverCert.DNSNames)
		suite.Assert().Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", serverCert.IPAddresses))

		suite.Assert().Equal("bar.example.com", serverCert.Subject.CommonName)
		suite.Assert().Empty(serverCert.Subject.Organization)

		suite.Assert().Equal(
			stdlibx509.KeyUsageDigitalSignature,
			serverCert.KeyUsage,
		)
		suite.Assert().Equal(
			[]stdlibx509.ExtKeyUsage{
				stdlibx509.ExtKeyUsageServerAuth,
				stdlibx509.ExtKeyUsageClientAuth,
			},
			serverCert.ExtKeyUsage,
		)

		suite.Assert().Nil(apiCerts.Client)

		return nil
	})
}
