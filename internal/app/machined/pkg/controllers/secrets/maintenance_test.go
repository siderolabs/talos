// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	stdlibx509 "crypto/x509"
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestMaintenanceSuite(t *testing.T) {
	suite.Run(t, &MaintenanceSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 2 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.MaintenanceController{}))
			},
		},
	})
}

type MaintenanceSuite struct {
	ctest.DefaultSuite
}

func (suite *MaintenanceSuite) TestReconcile() {
	rootSecrets := secrets.NewMaintenanceRoot(secrets.MaintenanceRootID)

	rootCA, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("talos"),
	)
	suite.Require().NoError(err)

	rootSecrets.TypedSpec().CA = &x509.PEMEncodedCertificateAndKey{
		Crt: rootCA.CrtPEM,
		Key: rootCA.KeyPEM,
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), rootSecrets))

	certSANs := secrets.NewCertSAN(secrets.NamespaceName, secrets.CertSANMaintenanceID)
	certSANs.TypedSpec().Append(
		"example.com",
		"foo",
		"10.2.1.3",
	)

	certSANs.TypedSpec().FQDN = "maintenance-service"
	suite.Require().NoError(suite.State().Create(suite.Ctx(), certSANs))

	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []resource.ID{secrets.MaintenanceServiceCertsID},
		func(certs *secrets.MaintenanceServiceCerts, asrt *assert.Assertions) {
			spec := certs.TypedSpec()

			asrt.Equal(rootCA.CrtPEM, spec.CA.Crt)
			asrt.Nil(spec.CA.Key)

			serverCert, err := spec.Server.GetCert()
			asrt.NoError(err)

			if err != nil {
				return
			}

			asrt.Equal([]string{"example.com", "foo"}, serverCert.DNSNames)
			asrt.Equal("[10.2.1.3]", fmt.Sprintf("%v", serverCert.IPAddresses))

			asrt.Equal("maintenance-service", serverCert.Subject.CommonName)
			asrt.Empty(serverCert.Subject.Organization)

			asrt.Equal(
				stdlibx509.KeyUsageDigitalSignature,
				serverCert.KeyUsage,
			)
			asrt.Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, serverCert.ExtKeyUsage)
		})
}
