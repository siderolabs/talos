// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider

import (
	stdlibtls "crypto/tls"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

type CertificateProviderSuite struct {
	suite.Suite
}

func TestCertificateProviderSuite(t *testing.T) {
	suite.Run(t, new(CertificateProviderSuite))
}

func (suite *CertificateProviderSuite) newAPICerts(withClient bool) (*secrets.API, []byte) {
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

	return api, ca.CrtPEM
}

func (suite *CertificateProviderSuite) TestUpdate() {
	api, caPEM := suite.newAPICerts(true)

	p := &certificateProvider{}
	suite.Require().NoError(p.Update(api))

	suite.True(p.HasClientCertificate())

	caBytes, err := p.GetCA()
	suite.Require().NoError(err)
	suite.Equal(caPEM, caBytes)

	pool, err := p.GetCACertPool()
	suite.Require().NoError(err)
	suite.NotNil(pool)

	serverCert, err := p.GetCertificate(nil)
	suite.Require().NoError(err)
	suite.NotNil(serverCert)

	clientCert, err := p.GetClientCertificate(nil)
	suite.Require().NoError(err)
	suite.NotNil(clientCert)
}

func (suite *CertificateProviderSuite) TestUpdateNoClient() {
	api, _ := suite.newAPICerts(false)

	p := &certificateProvider{}
	suite.Require().NoError(p.Update(api))

	suite.False(p.HasClientCertificate())

	clientCert, err := p.GetClientCertificate(nil)
	suite.Require().NoError(err)
	suite.Nil(clientCert)

	serverCert, err := p.GetCertificate(nil)
	suite.Require().NoError(err)
	suite.NotNil(serverCert)
}

func (suite *CertificateProviderSuite) TestUpdateInvalidServerKeyPair() {
	api, _ := suite.newAPICerts(true)
	api.TypedSpec().Server.Key = []byte("not-a-key")

	p := &certificateProvider{}
	suite.Error(p.Update(api))
}

func (suite *CertificateProviderSuite) TestUpdateInvalidClientKeyPair() {
	api, _ := suite.newAPICerts(true)
	api.TypedSpec().Client.Key = []byte("not-a-key")

	p := &certificateProvider{}
	suite.Error(p.Update(api))
}

func (suite *CertificateProviderSuite) TestUpdateInvalidCA() {
	api, _ := suite.newAPICerts(true)
	api.TypedSpec().AcceptedCAs = []*x509.PEMEncodedCertificate{
		{Crt: []byte("not-a-cert")},
	}

	p := &certificateProvider{}
	suite.Error(p.Update(api))
}

func (suite *CertificateProviderSuite) TestUpdateEmptyCA() {
	api, _ := suite.newAPICerts(true)
	api.TypedSpec().AcceptedCAs = nil

	p := &certificateProvider{}
	suite.Require().NoError(p.Update(api))

	caBytes, err := p.GetCA()
	suite.Require().NoError(err)
	suite.Empty(caBytes)
}

func (suite *CertificateProviderSuite) TestUpdateClearsClient() {
	p := &certificateProvider{}

	apiWithClient, _ := suite.newAPICerts(true)
	suite.Require().NoError(p.Update(apiWithClient))
	suite.True(p.HasClientCertificate())

	apiWithoutClient, _ := suite.newAPICerts(false)
	suite.Require().NoError(p.Update(apiWithoutClient))
	suite.False(p.HasClientCertificate())

	clientCert, err := p.GetClientCertificate(nil)
	suite.Require().NoError(err)
	suite.Nil(clientCert)
}

func (suite *CertificateProviderSuite) TestUpdateRotation() {
	p := &certificateProvider{}

	api1, _ := suite.newAPICerts(true)
	suite.Require().NoError(p.Update(api1))

	server1, err := p.GetCertificate(nil)
	suite.Require().NoError(err)

	client1, err := p.GetClientCertificate(nil)
	suite.Require().NoError(err)

	api2, _ := suite.newAPICerts(true)
	suite.Require().NoError(p.Update(api2))

	server2, err := p.GetCertificate(nil)
	suite.Require().NoError(err)
	suite.NotEqual(server1.Certificate[0], server2.Certificate[0])

	client2, err := p.GetClientCertificate(nil)
	suite.Require().NoError(err)
	suite.NotEqual(client1.Certificate[0], client2.Certificate[0])
}

func (suite *CertificateProviderSuite) newTLSConfig(withClient, skipClientCertVerify bool) *TLSConfig {
	api, _ := suite.newAPICerts(withClient)

	p := &certificateProvider{}
	suite.Require().NoError(p.Update(api))

	return &TLSConfig{
		certificateProvider:  p,
		skipClientCertVerify: skipClientCertVerify,
	}
}

func (suite *CertificateProviderSuite) TestServerConfigMutual() {
	cfg := suite.newTLSConfig(true, false)

	serverTLS, err := cfg.ServerConfig()
	suite.Require().NoError(err)
	suite.Require().NotNil(serverTLS)

	suite.Equal(stdlibtls.RequireAndVerifyClientCert, serverTLS.ClientAuth)
	suite.NotNil(serverTLS.GetCertificate)
	suite.NotNil(serverTLS.GetConfigForClient)

	serverCert, err := serverTLS.GetCertificate(nil)
	suite.Require().NoError(err)
	suite.NotNil(serverCert)
}

func (suite *CertificateProviderSuite) TestServerConfigServerOnly() {
	cfg := suite.newTLSConfig(true, true)

	serverTLS, err := cfg.ServerConfig()
	suite.Require().NoError(err)
	suite.Require().NotNil(serverTLS)

	suite.Equal(stdlibtls.NoClientCert, serverTLS.ClientAuth)
	suite.NotNil(serverTLS.GetCertificate)

	serverCert, err := serverTLS.GetCertificate(nil)
	suite.Require().NoError(err)
	suite.NotNil(serverCert)
}

func (suite *CertificateProviderSuite) TestClientConfigWithClient() {
	cfg := suite.newTLSConfig(true, false)

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Require().NotNil(clientTLS)

	suite.NotNil(clientTLS.GetClientCertificate)
	suite.NotNil(clientTLS.RootCAs)

	clientCert, err := clientTLS.GetClientCertificate(nil)
	suite.Require().NoError(err)
	suite.NotNil(clientCert)
}

func (suite *CertificateProviderSuite) TestClientConfigWithoutClient() {
	cfg := suite.newTLSConfig(false, false)

	clientTLS, err := cfg.ClientConfig()
	suite.Require().NoError(err)
	suite.Nil(clientTLS)
}
