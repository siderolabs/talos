// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubeconfig_test

import (
	"bytes"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/suite"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/siderolabs/talos/pkg/kubeconfig"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

type GenerateSuite struct {
	suite.Suite
}

func (suite *GenerateSuite) TestGenerateAdmin() {
	for _, rsa := range []bool{true, false} {
		suite.Run(fmt.Sprintf("RSA=%v", rsa), func() {
			ca, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(rsa))
			suite.Require().NoError(err)

			u, err := url.Parse("http://localhost:3333/api")
			suite.Require().NoError(err)

			cfg := &v1alpha1.ClusterConfig{
				ClusterName: "talos1",
				ClusterCA: &x509.PEMEncodedCertificateAndKey{
					Crt: ca.CrtPEM,
					Key: ca.KeyPEM,
				},
				ControlPlane: &v1alpha1.ControlPlaneConfig{
					Endpoint: &v1alpha1.Endpoint{
						URL: u,
					},
				},
				AdminKubeconfigConfig: &v1alpha1.AdminKubeconfigConfig{
					AdminKubeconfigCertLifetime: time.Hour,
				},
			}

			var buf bytes.Buffer

			suite.Require().NoError(kubeconfig.GenerateAdmin(cfg, &buf))

			// verify config via k8s client
			config, err := clientcmd.Load(buf.Bytes())
			suite.Require().NoError(err)

			suite.Assert().NoError(clientcmd.ConfirmUsable(*config, fmt.Sprintf("admin@%s", cfg.ClusterName)))
		})
	}
}

func (suite *GenerateSuite) TestGenerate() {
	ca, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(false))
	suite.Require().NoError(err)

	k8sCA := x509.NewCertificateAndKeyFromCertificateAuthority(ca)

	input := kubeconfig.GenerateInput{
		ClusterName: "foo",

		IssuingCA:           k8sCA,
		AcceptedCAs:         []*x509.PEMEncodedCertificate{{Crt: k8sCA.Crt}},
		CertificateLifetime: time.Hour,

		CommonName:   "system:kube-controller-manager",
		Organization: "system:kube-controller-manager",

		Endpoint:    "https://localhost:6443/",
		Username:    "kube-controller-manager",
		ContextName: "kube-controller-manager",
	}

	var buf bytes.Buffer

	suite.Require().NoError(kubeconfig.Generate(&input, &buf))

	// verify config via k8s client
	config, err := clientcmd.Load(buf.Bytes())
	suite.Require().NoError(err)

	suite.Assert().NoError(clientcmd.ConfirmUsable(*config, "kube-controller-manager@foo"))
}

func TestGenerateSuite(t *testing.T) {
	suite.Run(t, new(GenerateSuite))
}
