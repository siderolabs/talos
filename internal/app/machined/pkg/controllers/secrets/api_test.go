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

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

func TestAPISuite(t *testing.T) {
	suite.Run(t, &APISuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.APIController{}))
			},
		},
	})
}

type APISuite struct {
	ctest.DefaultSuite
}

func (suite *APISuite) TestReconcileControlPlane() {
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
	suite.Create(rootSecrets)

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Create(machineType)

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	suite.Create(networkStatus)

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
	suite.Create(certSANs)

	ctest.AssertResource(
		suite, secrets.APIID,
		func(certs *secrets.API, asrt *assert.Assertions) {
			apiCerts := certs.TypedSpec()

			asrt.Equal(
				[]*x509.PEMEncodedCertificate{
					{
						Crt: talosCA.CrtPEM,
					},
				},
				apiCerts.AcceptedCAs,
			)

			serverCert, err := apiCerts.Server.GetCert()
			if !asrt.NoError(err) {
				return
			}

			asrt.Equal([]string{"example.com", "foo", "foo.example.com"}, serverCert.DNSNames)
			asrt.Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", serverCert.IPAddresses))

			asrt.Equal("foo.example.com", serverCert.Subject.CommonName)
			asrt.Empty(serverCert.Subject.Organization)

			asrt.Equal(
				stdlibx509.KeyUsageDigitalSignature,
				serverCert.KeyUsage,
			)
			asrt.Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, serverCert.ExtKeyUsage)

			clientCert, err := apiCerts.Client.GetCert()
			if !asrt.NoError(err) {
				return
			}

			asrt.Empty(clientCert.DNSNames)
			asrt.Empty(clientCert.IPAddresses)

			asrt.Equal("foo.example.com", clientCert.Subject.CommonName)
			asrt.Equal([]string{string(role.Impersonator)}, clientCert.Subject.Organization)

			asrt.Equal(
				stdlibx509.KeyUsageDigitalSignature,
				clientCert.KeyUsage,
			)
			asrt.Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageClientAuth}, clientCert.ExtKeyUsage)

			asrt.False(apiCerts.SkipVerifyingClientCert)
		},
	)

	// destroy machine type, mocking transition to maintenance mode
	suite.Destroy(machineType)

	ctest.AssertNoResource[*secrets.API](suite, secrets.APIID)
}

func (suite *APISuite) TestReconcileMaintenance() {
	suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.MaintenanceRootController{}))

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	suite.Create(networkStatus)

	certSANs := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANMaintenanceID)
	certSANs.TypedSpec().Append(
		"example.com",
		"10.2.1.3",
	)
	certSANs.TypedSpec().FQDN = constants.MaintenanceServiceCommonName
	suite.Create(certSANs)

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeUnknown)
	suite.Create(machineType)

	ctest.AssertResource(
		suite, secrets.APIID,
		func(certs *secrets.API, asrt *assert.Assertions) {
			apiCerts := certs.TypedSpec()

			asrt.True(apiCerts.SkipVerifyingClientCert)
			asrt.Nil(apiCerts.Client)
			asrt.Nil(apiCerts.AcceptedCAs)

			serverCert, err := apiCerts.Server.GetCert()
			if !asrt.NoError(err) {
				return
			}

			asrt.Equal([]string{"example.com"}, serverCert.DNSNames)
			asrt.Equal("[10.2.1.3]", fmt.Sprintf("%v", serverCert.IPAddresses))

			asrt.Equal(constants.MaintenanceServiceCommonName, serverCert.Subject.CommonName)
			asrt.Empty(serverCert.Subject.Organization)

			asrt.Equal(
				stdlibx509.KeyUsageDigitalSignature,
				serverCert.KeyUsage,
			)
			asrt.Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, serverCert.ExtKeyUsage)
		},
	)

	// create machine type, mocking transition to control plane mode
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Update(machineType)

	ctest.AssertNoResource[*secrets.API](suite, secrets.APIID)
}
