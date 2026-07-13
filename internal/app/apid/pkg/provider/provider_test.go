// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider_test

import (
	"context"
	stdlibtls "crypto/tls"
	stdx509 "crypto/x509"
	"encoding/pem"
	"slices"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/apid/pkg/provider"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

// Named arguments for the helpers below: both parameters are booleans, so spelling
// them out at the call site keeps the cases readable.
const (
	withClientCert    = true
	withoutClientCert = false

	verifyClientCert     = false
	skipClientCertVerify = true
)

// watch tracks a TLSConfig.Watch running in the background.
type watch struct {
	updated chan struct{}
	errCh   chan error
	joined  bool
}

type TLSConfigSuite struct {
	suite.Suite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	resources state.State
	watches   []*watch
}

func TestTLSConfigSuite(t *testing.T) {
	suite.Run(t, new(TLSConfigSuite))
}

func (suite *TLSConfigSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(suite.T().Context(), 10*time.Second)
	suite.resources = state.WrapCore(namespaced.NewState(inmem.Build))
	suite.watches = nil
}

func (suite *TLSConfigSuite) TearDownTest() {
	suite.ctxCancel()

	// Watch returns nil once the context is canceled, and it should not outlive the test.
	for _, w := range suite.watches {
		if w.joined {
			continue
		}

		select {
		case err := <-w.errCh:
			suite.NoError(err, "watch should return nil once the context is canceled")
		case <-time.After(10 * time.Second):
			suite.Fail("watch goroutine didn't stop after the context was canceled")
		}
	}
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

// newTLSConfig seeds the state with the given API certs and returns a ready TLSConfig.
func (suite *TLSConfigSuite) newTLSConfig(api *secrets.API, skipClientCertVerify bool) *provider.TLSConfig {
	suite.Require().NoError(suite.resources.Create(suite.ctx, api))

	cfg, err := provider.NewTLSConfig(suite.ctx, suite.resources, skipClientCertVerify)
	suite.Require().NoError(err)

	return cfg
}

// updateAPI applies mutate to the current secrets.API resource in state.
func (suite *TLSConfigSuite) updateAPI(mutate func(*secrets.API)) {
	api, err := safe.StateGetResource(suite.ctx, suite.resources, secrets.NewAPI())
	suite.Require().NoError(err)

	mutate(api)
	suite.Require().NoError(suite.resources.Update(suite.ctx, api))
}

// startWatch runs TLSConfig.Watch in the background; the returned watch is joined in TearDownTest.
func (suite *TLSConfigSuite) startWatch(cfg *provider.TLSConfig) *watch {
	w := &watch{
		updated: make(chan struct{}, 1),
		errCh:   make(chan error, 1),
	}

	suite.watches = append(suite.watches, w)

	go func() {
		w.errCh <- cfg.Watch(suite.ctx, func() {
			select {
			case w.updated <- struct{}{}:
			default:
			}
		})
	}()

	return w
}

// waitUpdate blocks until the watch reports an update; if the watch returns first, its
// error is reported instead of letting the test hit the context timeout.
func (suite *TLSConfigSuite) waitUpdate(w *watch) {
	select {
	case <-w.updated:
	case err := <-w.errCh:
		w.joined = true

		suite.Require().NoError(err, "watch returned before the update landed")
		suite.Require().Fail("watch returned before the update landed")
	case <-suite.ctx.Done():
		suite.Require().Fail("timed out waiting for watch update")
	}
}

// waitError blocks until the watch returns and yields its error.
func (suite *TLSConfigSuite) waitError(w *watch) error {
	select {
	case err := <-w.errCh:
		w.joined = true

		return err
	case <-suite.ctx.Done():
		suite.Require().Fail("timed out waiting for the watch to return")

		return nil
	}
}

// leafDER decodes a PEM-encoded certificate into its DER bytes, as they appear in tls.Certificate.
func (suite *TLSConfigSuite) leafDER(crtPEM []byte) []byte {
	block, rest := pem.Decode(crtPEM)
	suite.Require().NotNil(block)
	suite.Require().Equal("CERTIFICATE", block.Type)
	suite.Require().Empty(rest)

	return block.Bytes
}

// certPool builds the cert pool expected for the given set of accepted CAs.
func (suite *TLSConfigSuite) certPool(acceptedCAs []*x509.PEMEncodedCertificate) *stdx509.CertPool {
	pool := stdx509.NewCertPool()

	for _, ca := range acceptedCAs {
		suite.Require().True(pool.AppendCertsFromPEM(ca.Crt))
	}

	return pool
}

// serverCertDER returns the DER of the certificate the server config serves right now.
func (suite *TLSConfigSuite) serverCertDER(serverTLS *stdlibtls.Config) []byte {
	cert, err := serverTLS.GetCertificate(nil)
	suite.Require().NoError(err)
	suite.Require().NotNil(cert)
	suite.Require().NotEmpty(cert.Certificate)

	return cert.Certificate[0]
}

// clientCertDER returns the DER of the certificate the client config presents right now.
func (suite *TLSConfigSuite) clientCertDER(clientTLS *stdlibtls.Config) []byte {
	cert, err := clientTLS.GetClientCertificate(nil)
	suite.Require().NoError(err)
	suite.Require().NotNil(cert)
	suite.Require().NotEmpty(cert.Certificate)

	return cert.Certificate[0]
}

// TestServerConfigMutual checks mutual TLS client auth when skipClientCertVerify is false.
func (suite *TLSConfigSuite) TestServerConfigMutual() {
	api := suite.newAPICerts(withClientCert)
	cfg := suite.newTLSConfig(api, verifyClientCert)

	serverTLS, err := cfg.ServerConfig()
	suite.Require().NoError(err)
	suite.Equal(stdlibtls.RequireAndVerifyClientCert, serverTLS.ClientAuth)
	suite.Equal(suite.leafDER(api.TypedSpec().Server.Crt), suite.serverCertDER(serverTLS))

	// the client CA pool is only filled in per connection (dynamic client CA), so the base config carries an empty one
	suite.True(stdx509.NewCertPool().Equal(serverTLS.ClientCAs))

	cfgForClient, err := serverTLS.GetConfigForClient(nil)
	suite.Require().NoError(err)
	suite.Equal(stdlibtls.RequireAndVerifyClientCert, cfgForClient.ClientAuth)
	suite.True(suite.certPool(api.TypedSpec().AcceptedCAs).Equal(cfgForClient.ClientCAs))
}

// TestServerConfigServerOnly checks server-only TLS when skipClientCertVerify is true.
func (suite *TLSConfigSuite) TestServerConfigServerOnly() {
	api := suite.newAPICerts(withClientCert)
	cfg := suite.newTLSConfig(api, skipClientCertVerify)

	serverTLS, err := cfg.ServerConfig()
	suite.Require().NoError(err)
	suite.Equal(stdlibtls.NoClientCert, serverTLS.ClientAuth)
	suite.Equal(suite.leafDER(api.TypedSpec().Server.Crt), suite.serverCertDER(serverTLS))

	// client certs are not verified, but the per-connection config is still built the same way
	cfgForClient, err := serverTLS.GetConfigForClient(nil)
	suite.Require().NoError(err)
	suite.Equal(stdlibtls.NoClientCert, cfgForClient.ClientAuth)
	suite.True(suite.certPool(api.TypedSpec().AcceptedCAs).Equal(cfgForClient.ClientCAs))
}

// TestClientConfig checks that the client config presents the client cert and trusts the accepted CAs.
func (suite *TLSConfigSuite) TestClientConfig() {
	api := suite.newAPICerts(withClientCert)
	cfg := suite.newTLSConfig(api, verifyClientCert)

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Require().NotNil(clientTLS)

	suite.Equal(suite.leafDER(api.TypedSpec().Client.Crt), suite.clientCertDER(clientTLS))
	suite.True(suite.certPool(api.TypedSpec().AcceptedCAs).Equal(clientTLS.RootCAs))
}

// TestClientConfigWithoutClient checks that ClientConfig is nil without a client cert.
func (suite *TLSConfigSuite) TestClientConfigWithoutClient() {
	cfg := suite.newTLSConfig(suite.newAPICerts(withoutClientCert), verifyClientCert)

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Nil(clientTLS)
}

// TestClientConfigWithoutCA checks that an empty set of accepted CAs is tolerated by the
// certificate provider, but makes the client config unbuildable.
func (suite *TLSConfigSuite) TestClientConfigWithoutCA() {
	api := suite.newAPICerts(withClientCert)
	api.TypedSpec().AcceptedCAs = nil

	cfg := suite.newTLSConfig(api, verifyClientCert)

	_, err := cfg.ClientConfig()
	suite.Require().Error(err)
	suite.ErrorContains(err, "no CA cert provided")
}

// TestWatchRotation checks that Watch reloads server and client certs, and the accepted CAs, on API updates.
func (suite *TLSConfigSuite) TestWatchRotation() {
	api := suite.newAPICerts(withClientCert)
	cfg := suite.newTLSConfig(api, verifyClientCert)

	oldCAs := api.TypedSpec().AcceptedCAs

	serverTLS, err := cfg.ServerConfig()
	suite.Require().NoError(err)
	suite.Require().Equal(suite.leafDER(api.TypedSpec().Server.Crt), suite.serverCertDER(serverTLS))

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Require().Equal(suite.leafDER(api.TypedSpec().Client.Crt), suite.clientCertDER(clientTLS))

	w := suite.startWatch(cfg)

	// Refresh the secrets.API object with a new one: it rotates the keys and certs, and
	// brings in a new CA which is accepted alongside the old one (as on CA rotation).
	next := suite.newAPICerts(withClientCert)
	next.TypedSpec().AcceptedCAs = slices.Concat(oldCAs, next.TypedSpec().AcceptedCAs)

	suite.updateAPI(func(api *secrets.API) {
		*api.TypedSpec() = *next.TypedSpec()
	})

	suite.waitUpdate(w)

	// The server and client certs are served through the certificate provider, so the
	// configs built before the rotation now hand out exactly the new certs.
	suite.Equal(suite.leafDER(next.TypedSpec().Server.Crt), suite.serverCertDER(serverTLS))
	suite.Equal(suite.leafDER(next.TypedSpec().Client.Crt), suite.clientCertDER(clientTLS))

	// The client CA pool on the server side is dynamic as well: a new connection is
	// verified against both the old and the new accepted CA.
	cfgForClient, err := serverTLS.GetConfigForClient(nil)
	suite.Require().NoError(err)
	suite.True(suite.certPool(next.TypedSpec().AcceptedCAs).Equal(cfgForClient.ClientCAs))

	// The client-side CA pool, on the other hand, is a snapshot taken when ClientConfig
	// was called, so this config still trusts the old CA only: apid depends on
	// APIDFactory.Flush (wired as onPKIUpdate in internal/app/apid/service.go) dropping
	// the cached backends so that ClientConfig is called again.
	suite.True(suite.certPool(oldCAs).Equal(clientTLS.RootCAs))

	// A client config built after the rotation trusts both CAs.
	rotatedClientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.True(suite.certPool(next.TypedSpec().AcceptedCAs).Equal(rotatedClientTLS.RootCAs))
}

// TestWatchClearsClient checks that removing the client cert makes ClientConfig nil.
func (suite *TLSConfigSuite) TestWatchClearsClient() {
	api := suite.newAPICerts(withClientCert)
	cfg := suite.newTLSConfig(api, verifyClientCert)

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Require().NotNil(clientTLS, "there should be a client config before the client cert is removed")
	suite.Require().Equal(suite.leafDER(api.TypedSpec().Client.Crt), suite.clientCertDER(clientTLS))

	w := suite.startWatch(cfg)

	suite.updateAPI(func(api *secrets.API) {
		api.TypedSpec().Client = nil
	})

	suite.waitUpdate(w)

	clientTLS, err = cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Nil(clientTLS)
}

// TestWatchInvalidCA checks that Watch fails, without reporting an update, when the
// accepted CAs can't be parsed.
func (suite *TLSConfigSuite) TestWatchInvalidCA() {
	cfg := suite.newTLSConfig(suite.newAPICerts(withClientCert), verifyClientCert)

	w := suite.startWatch(cfg)

	suite.updateAPI(func(api *secrets.API) {
		api.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{{Crt: []byte("not a PEM certificate")}}
	})

	err := suite.waitError(w)
	suite.Require().Error(err)
	suite.ErrorContains(err, "failed to parse CA certs into a CertPool")

	select {
	case <-w.updated:
		suite.Fail("onUpdate should not be called when the update fails")
	default:
	}
}

// TestNewTLSConfigInvalidServerCert checks that NewTLSConfig fails on an unusable server keypair.
func (suite *TLSConfigSuite) TestNewTLSConfigInvalidServerCert() {
	api := suite.newAPICerts(withoutClientCert)
	api.TypedSpec().Server = &x509.PEMEncodedCertificateAndKey{Crt: []byte("not a PEM certificate")}

	suite.Require().NoError(suite.resources.Create(suite.ctx, api))

	_, err := provider.NewTLSConfig(suite.ctx, suite.resources, verifyClientCert)
	suite.Require().Error(err)
	suite.ErrorContains(err, "failed to parse server cert and key")
}

// TestNewTLSConfigContextCanceled checks that NewTLSConfig gives up when the context is
// canceled before the first secrets.API event arrives.
func (suite *TLSConfigSuite) TestNewTLSConfigContextCanceled() {
	ctx, cancel := context.WithCancel(suite.ctx)
	cancel()

	// nothing was created in the state, so there's no event to receive
	_, err := provider.NewTLSConfig(ctx, suite.resources, verifyClientCert)
	suite.Require().Error(err)
	suite.ErrorIs(err, context.Canceled)
}
