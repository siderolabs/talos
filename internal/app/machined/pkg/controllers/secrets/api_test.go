// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package secrets_test

import (
	"context"
	stdlibx509 "crypto/x509"
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller/runtime"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/go-retry/retry"
	"inet.af/netaddr"

	secretsctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/talos-systems/talos/pkg/logging"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
	"github.com/talos-systems/talos/pkg/machinery/role"
)

type APISuite struct {
	suite.Suite

	state state.State

	runtime *runtime.Runtime
	wg      sync.WaitGroup

	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (suite *APISuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	suite.state = state.WrapCore(namespaced.NewState(inmem.Build))

	var err error

	suite.runtime, err = runtime.NewRuntime(suite.state, logging.Wrap(log.Writer()))
	suite.Require().NoError(err)

	suite.Require().NoError(suite.runtime.RegisterController(&secretsctrl.APIController{}))

	suite.startRuntime()
}

func (suite *APISuite) startRuntime() {
	suite.wg.Add(1)

	go func() {
		defer suite.wg.Done()

		suite.Assert().NoError(suite.runtime.Run(suite.ctx))
	}()
}

func (suite *APISuite) TestReconcileControlPlane() {
	rootSecrets := secrets.NewOSRoot(secrets.OSRootID)

	talosCA, err := x509.NewSelfSignedCertificateAuthority(
		x509.Organization("talos"),
	)
	suite.Require().NoError(err)

	rootSecrets.TypedSpec().CA = &x509.PEMEncodedCertificateAndKey{
		Crt: talosCA.CrtPEM,
		Key: talosCA.KeyPEM,
	}
	rootSecrets.TypedSpec().CertSANDNSNames = []string{"example.com"}
	rootSecrets.TypedSpec().CertSANIPs = []netaddr.IP{netaddr.MustParseIP("10.4.3.2"), netaddr.MustParseIP("10.2.1.3")}
	rootSecrets.TypedSpec().Token = "something"
	suite.Require().NoError(suite.state.Create(suite.ctx, rootSecrets))

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeControlPlane)
	suite.Require().NoError(suite.state.Create(suite.ctx, machineType))

	networkStatus := network.NewStatus(network.NamespaceName, network.StatusID)
	networkStatus.TypedSpec().AddressReady = true
	networkStatus.TypedSpec().HostnameReady = true
	suite.Require().NoError(suite.state.Create(suite.ctx, networkStatus))

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

	suite.Require().NoError(suite.state.Create(suite.ctx, certSANs))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			certs, err := suite.state.Get(suite.ctx, resource.NewMetadata(secrets.NamespaceName, secrets.APIType, secrets.APIID, resource.VersionUndefined))
			if err != nil {
				if state.IsNotFoundError(err) {
					return retry.ExpectedError(err)
				}

				return err
			}

			apiCerts := certs.(*secrets.API).TypedSpec()

			suite.Assert().Equal(talosCA.CrtPEM, apiCerts.CA.Crt)
			suite.Assert().Nil(apiCerts.CA.Key)

			serverCert, err := apiCerts.Server.GetCert()
			suite.Require().NoError(err)

			suite.Assert().Equal([]string{"example.com", "foo", "foo.example.com"}, serverCert.DNSNames)
			suite.Assert().Equal("[10.2.1.3 10.4.3.2 172.16.0.1]", fmt.Sprintf("%v", serverCert.IPAddresses))

			suite.Assert().Equal("foo.example.com", serverCert.Subject.CommonName)
			suite.Assert().Empty(serverCert.Subject.Organization)

			suite.Assert().Equal(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment, serverCert.KeyUsage)
			suite.Assert().Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageServerAuth}, serverCert.ExtKeyUsage)

			clientCert, err := apiCerts.Client.GetCert()
			suite.Require().NoError(err)

			suite.Assert().Empty(clientCert.DNSNames)
			suite.Assert().Empty(clientCert.IPAddresses)

			suite.Assert().Equal("foo.example.com", clientCert.Subject.CommonName)
			suite.Assert().Equal([]string{string(role.Impersonator)}, clientCert.Subject.Organization)

			suite.Assert().Equal(stdlibx509.KeyUsageDigitalSignature|stdlibx509.KeyUsageKeyEncipherment, clientCert.KeyUsage)
			suite.Assert().Equal([]stdlibx509.ExtKeyUsage{stdlibx509.ExtKeyUsageClientAuth}, clientCert.ExtKeyUsage)

			return nil
		},
	))
}

func (suite *APISuite) TearDownTest() {
	suite.T().Log("tear down")

	suite.ctxCancel()

	suite.wg.Wait()
}

func TestAPISuite(t *testing.T) {
	suite.Run(t, new(APISuite))
}
