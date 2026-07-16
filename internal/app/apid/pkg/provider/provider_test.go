// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider_test

import (
	"context"
	stdlibtls "crypto/tls"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/apid/pkg/provider"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

type TLSConfigSuite struct {
	suite.Suite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	resources state.State
}

func TestTLSConfigSuite(t *testing.T) {
	suite.Run(t, new(TLSConfigSuite))
}

func (suite *TLSConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.T().Context(), 10*time.Second)
	suite.resources = state.WrapCore(namespaced.NewState(inmem.Build))
}

func (suite *TLSConfigSuite) TearDownTest() {
	suite.ctxCancel()
}

// newAPICerts builds a secrets.API resource with a fresh CA and server cert.
// When withClient is true, a client cert is included as well.
func (suite *TLSConfigSuite) newAPICerts(withClient bool) *secrets.API {
	ca, err := x509.NewSelfSignedCertificateAuthority()
	suite.Require().NoError(err)

	serverCrt, err := x509.NewKeyPair(ca)
	suite.Require().NoError(err)

	api := secrets.NewAPI()
	api.TypedSpec().Server = x509.NewCertificateAndKeyFromKeyPair(serverCrt)
	api.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{{Crt: ca.CrtPEM}}

	if withClient {
		clientCrt, err := x509.NewKeyPair(ca)
		suite.Require().NoError(err)

		api.TypedSpec().Client = x509.NewCertificateAndKeyFromKeyPair(clientCrt)
	}

	return api
}

// newTLSConfig seeds the state with API certs and returns a ready TLSConfig.
func (suite *TLSConfigSuite) newTLSConfig(withClient, skipClientCertVerify bool) *provider.TLSConfig {
	suite.Require().NoError(suite.resources.Create(suite.ctx, suite.newAPICerts(withClient)))

	cfg, err := provider.NewTLSConfig(suite.ctx, suite.resources, skipClientCertVerify)
	suite.Require().NoError(err)

	return cfg
}

// updateAPI applies mutate to the current secrets.API resource in state.
func (suite *TLSConfigSuite) updateAPI(mutate func(*secrets.API)) {
	r, err := suite.resources.Get(suite.ctx, resource.NewMetadata(secrets.NamespaceName, secrets.APIType, secrets.APIID, resource.VersionUndefined))
	suite.Require().NoError(err)

	api := r.(*secrets.API).DeepCopy().(*secrets.API) //nolint:forcetypeassert
	mutate(api)
	suite.Require().NoError(suite.resources.Update(suite.ctx, api))
}

// TestServerConfigMutual checks mutual TLS client auth when skipClientCertVerify is false.
func (suite *TLSConfigSuite) TestServerConfigMutual() {
	cfg := suite.newTLSConfig(true, false)

	serverTLS, err := cfg.ServerConfig()
	suite.Require().NoError(err)
	suite.Equal(stdlibtls.RequireAndVerifyClientCert, serverTLS.ClientAuth)

	serverCert, err := serverTLS.GetCertificate(nil)
	suite.Require().NoError(err)
	suite.NotNil(serverCert)
}

// TestServerConfigServerOnly checks server-only TLS when skipClientCertVerify is true.
func (suite *TLSConfigSuite) TestServerConfigServerOnly() {
	cfg := suite.newTLSConfig(true, true)

	serverTLS, err := cfg.ServerConfig()
	suite.Require().NoError(err)
	suite.Equal(stdlibtls.NoClientCert, serverTLS.ClientAuth)
}

// TestClientConfigWithClient checks ClientConfig when a client cert is present.
func (suite *TLSConfigSuite) TestClientConfigWithClient() {
	cfg := suite.newTLSConfig(true, false)

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Require().NotNil(clientTLS)

	clientCert, err := clientTLS.GetClientCertificate(nil)
	suite.Require().NoError(err)
	suite.NotNil(clientCert)
}

// TestClientConfigWithoutClient checks that ClientConfig is nil without a client cert.
func (suite *TLSConfigSuite) TestClientConfigWithoutClient() {
	cfg := suite.newTLSConfig(false, false)

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Nil(clientTLS)
}

// TestWatchRotation checks that Watch reloads server and client certs on API updates.
func (suite *TLSConfigSuite) TestWatchRotation() {
	cfg := suite.newTLSConfig(true, false)

	serverTLS, err := cfg.ServerConfig()
	suite.Require().NoError(err)

	server1, err := serverTLS.GetCertificate(nil)
	suite.Require().NoError(err)

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)

	client1, err := clientTLS.GetClientCertificate(nil)
	suite.Require().NoError(err)

	updated := make(chan struct{}, 1)

	go func() {
		_ = cfg.Watch(suite.ctx, func() {
			select {
			case updated <- struct{}{}:
			default:
			}
		})
	}()

	next := suite.newAPICerts(true)
	suite.updateAPI(func(api *secrets.API) {
		*api.TypedSpec() = *next.TypedSpec()
	})

	select {
	case <-updated:
	case <-suite.ctx.Done():
		suite.Require().Fail("timed out waiting for watch update")
	}

	server2, err := serverTLS.GetCertificate(nil)
	suite.Require().NoError(err)
	suite.NotEqual(server1.Certificate[0], server2.Certificate[0])

	client2, err := clientTLS.GetClientCertificate(nil)
	suite.Require().NoError(err)
	suite.NotEqual(client1.Certificate[0], client2.Certificate[0])
}

// TestWatchClearsClient checks that removing the client cert makes ClientConfig nil.
func (suite *TLSConfigSuite) TestWatchClearsClient() {
	cfg := suite.newTLSConfig(true, false)

	updated := make(chan struct{}, 1)

	go func() {
		_ = cfg.Watch(suite.ctx, func() {
			select {
			case updated <- struct{}{}:
			default:
			}
		})
	}()

	suite.updateAPI(func(api *secrets.API) {
		api.TypedSpec().Client = nil
	})

	select {
	case <-updated:
	case <-suite.ctx.Done():
		suite.Require().Fail("timed out waiting for watch update")
	}

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Nil(clientTLS)
}
