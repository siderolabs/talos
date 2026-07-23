// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"net/url"
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	k8scfg "github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestKubeletSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KubeletSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(secretsctrl.NewKubeletController()))
			},
		},
	})
}

type KubeletSuite struct {
	ctest.DefaultSuite
}

func (suite *KubeletSuite) TestReconcile() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(false))
	suite.Require().NoError(err)

	k8sCA := x509.NewCertificateAndKeyFromCertificateAuthority(ca)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{},
				ClusterConfig: &v1alpha1.ClusterConfig{
					ControlPlane: &v1alpha1.ControlPlaneConfig{
						Endpoint: &v1alpha1.Endpoint{
							URL: u,
						},
					},
					ClusterCA:      k8sCA,
					BootstrapToken: "abc.def",
				},
			},
		),
	)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, secrets.KubeletID, func(kubeletSecrets *secrets.Kubelet, asrt *assert.Assertions) {
		spec := kubeletSecrets.TypedSpec()

		suite.Assert().Equal("https://foo:6443", spec.Endpoint.String())
		suite.Assert().Equal([]*x509.PEMEncodedCertificate{{Crt: k8sCA.Crt}}, spec.AcceptedCAs)
		suite.Assert().Equal("abc", spec.BootstrapTokenID)
		suite.Assert().Equal("def", spec.BootstrapTokenSecret)
		suite.Assert().Empty(spec.EndpointTLSServerName)
	})
}

func (suite *KubeletSuite) TestReconcileKubePrism() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(false))
	suite.Require().NoError(err)

	k8sCA := x509.NewCertificateAndKeyFromCertificateAuthority(ca)

	v1alpha1Cfg := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{},
		ClusterConfig: &v1alpha1.ClusterConfig{
			ControlPlane: &v1alpha1.ControlPlaneConfig{
				Endpoint: &v1alpha1.Endpoint{
					URL: u,
				},
			},
			ClusterCA:      k8sCA,
			BootstrapToken: "abc.def",
		},
	}

	kubePrismConfig := k8scfg.NewKubePrismConfigV1Alpha1()
	kubePrismConfig.PortConfig = 3333
	kubePrismConfig.TLSServerNameConfig = "my-lb"

	ctr, err := container.New(v1alpha1Cfg, kubePrismConfig)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), cfg))

	ctest.AssertResource(suite, secrets.KubeletID, func(kubeletSecrets *secrets.Kubelet, asrt *assert.Assertions) {
		spec := kubeletSecrets.TypedSpec()

		suite.Assert().Equal("https://127.0.0.1:3333", spec.Endpoint.String())
		suite.Assert().Equal([]*x509.PEMEncodedCertificate{{Crt: k8sCA.Crt}}, spec.AcceptedCAs)
		suite.Assert().Equal("abc", spec.BootstrapTokenID)
		suite.Assert().Equal("def", spec.BootstrapTokenSecret)
		suite.Assert().Equal("my-lb", spec.EndpointTLSServerName)
	})
}
