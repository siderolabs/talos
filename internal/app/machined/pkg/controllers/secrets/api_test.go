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
	suite.Run(t, &APISuite{
		DefaultSuite: ctest.DefaultSuite{},
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
}

func (suite *APISuite) TestReconcileControlPlanePureEd25519() {
	suite.testReconcileControlPlaneWithAlgo(stdlibx509.PureEd25519)
}

func (suite *APISuite) TestReconcileControlPlaneECDSAWithSHA256() {
	suite.testReconcileControlPlaneWithAlgo(stdlibx509.ECDSAWithSHA256)
}

func (suite *APISuite) TestReconcileControlPlaneECDSAWithSHA384() {
	suite.testReconcileControlPlaneWithAlgo(stdlibx509.ECDSAWithSHA384)
}

func (suite *APISuite) TestReconcileControlPlaneECDSAWithSHA512() {
	suite.testReconcileControlPlaneWithAlgo(stdlibx509.ECDSAWithSHA512)
}

func (suite *APISuite) TestReconcileControlPlaneSHA256WithRSA() {
	suite.testReconcileControlPlaneWithAlgo(stdlibx509.SHA256WithRSA)
}

func (suite *APISuite) TestReconcileControlPlaneSHA384WithRSA() {
	suite.testReconcileControlPlaneWithAlgo(stdlibx509.SHA384WithRSA)
}

func (suite *APISuite) TestReconcileControlPlaneSHA512WithRSA() {
	suite.testReconcileControlPlaneWithAlgo(stdlibx509.SHA512WithRSA)
}

func (suite *APISuite) TestReconcileWorkerPureEd25519() {
	suite.testReconcileWorkerWithAlgo(stdlibx509.PureEd25519)
}

func (suite *APISuite) TestReconcileWorkerECDSAWithSHA256() {
	suite.testReconcileWorkerWithAlgo(stdlibx509.ECDSAWithSHA256)
}

func (suite *APISuite) TestReconcileWorkerECDSAWithSHA384() {
	suite.testReconcileWorkerWithAlgo(stdlibx509.ECDSAWithSHA384)
}

func (suite *APISuite) TestReconcileWorkerECDSAWithSHA512() {
	suite.testReconcileWorkerWithAlgo(stdlibx509.ECDSAWithSHA512)
}

func (suite *APISuite) TestReconcileWorkerSHA256WithRSA() {
	suite.testReconcileWorkerWithAlgo(stdlibx509.SHA256WithRSA)
}

func (suite *APISuite) TestReconcileWorkerSHA384WithRSA() {
	suite.testReconcileWorkerWithAlgo(stdlibx509.SHA384WithRSA)
}

func (suite *APISuite) TestReconcileWorkerSHA512WithRSA() {
	suite.testReconcileWorkerWithAlgo(stdlibx509.SHA512WithRSA)
}

func (suite *APISuite) testReconcileControlPlaneWithAlgo(algorithm stdlibx509.SignatureAlgorithm) {
	talosCA, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("talos"),
		x509.SignatureAlgorithm(algorithm),
	)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.APIController{
		NewRemoteGenerator: func(token string, endpoints []string, acceptedCAs []*x509.PEMEncodedCertificate) (secretsctrl.CertificateGenerator, error) {
			g, err := gen.NewLocalGenerator(talosCA.KeyPEM, talosCA.CrtPEM)
			if err != nil {
				return nil, err
			}

			return mockGenerator{g: g}, nil
		},
	}))

	rootSecrets := secrets.NewOSRoot(secrets.OSRootID)

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
					Crt: talosCA.CrtPEM,
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

func (suite *APISuite) testReconcileWorkerWithAlgo(algorithm stdlibx509.SignatureAlgorithm) {
	talosCA, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("talos"),
		x509.SignatureAlgorithm(algorithm),
	)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.APIController{
		NewRemoteGenerator: func(token string, endpoints []string, acceptedCAs []*x509.PEMEncodedCertificate) (secretsctrl.CertificateGenerator, error) {
			g, err := gen.NewLocalGenerator(talosCA.KeyPEM, talosCA.CrtPEM)
			if err != nil {
				return nil, err
			}

			return mockGenerator{g: g}, nil
		},
	}))

	rootSecrets := secrets.NewOSRoot(secrets.OSRootID)

	rootSecrets.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{
		{
			Crt: talosCA.CrtPEM,
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
					Crt: talosCA.CrtPEM,
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
