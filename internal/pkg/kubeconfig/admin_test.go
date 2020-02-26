// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig_test

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"

	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/talos-systems/talos/internal/pkg/kubeconfig"
	"github.com/talos-systems/talos/pkg/crypto/x509"
)

type mockClusterConfig struct {
	name string
	ca   *x509.PEMEncodedCertificateAndKey
}

func (c mockClusterConfig) Name() string {
	return c.name
}

func (c mockClusterConfig) CA() *x509.PEMEncodedCertificateAndKey {
	return c.ca
}

func (c mockClusterConfig) Endpoint() *url.URL {
	u, _ := url.Parse("http://localhost:6443/api/") //nolint: errcheck

	return u
}

type AdminSuite struct {
	suite.Suite
}

func (suite *AdminSuite) TestGenerate() {
	ca, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(true))
	suite.Require().NoError(err)

	cfg := mockClusterConfig{
		name: "talos1",
		ca: &x509.PEMEncodedCertificateAndKey{
			Crt: ca.CrtPEM,
			Key: ca.KeyPEM,
		},
	}

	var buf bytes.Buffer

	suite.Require().NoError(kubeconfig.GenerateAdmin(cfg, &buf))

	// verify config via k8s client
	config, err := clientcmd.Load(buf.Bytes())
	suite.Require().NoError(err)

	suite.Assert().NoError(clientcmd.ConfirmUsable(*config, fmt.Sprintf("admin@%s", cfg.name)))
}

func TestAdminSuite(t *testing.T) {
	suite.Run(t, new(AdminSuite))
}
