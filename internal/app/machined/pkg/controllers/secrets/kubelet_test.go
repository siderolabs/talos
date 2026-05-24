// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/crypto/x509"
	"github.com/siderolabs/go-pointer"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestKubeletSuite(t *testing.T) {
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

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				kubeletSecrets, err := ctest.Get[*secrets.Kubelet](
					suite,
					resource.NewMetadata(
						secrets.NamespaceName,
						secrets.KubeletType,
						secrets.KubeletID,
						resource.VersionUndefined,
					),
				)
				if err != nil {
					if state.IsNotFoundError(err) {
						return retry.ExpectedError(err)
					}

					return err
				}

				spec := kubeletSecrets.TypedSpec()

				suite.Assert().Equal("https://foo:6443", spec.Endpoint.String())
				suite.Assert().Equal([]*x509.PEMEncodedCertificate{{Crt: k8sCA.Crt}}, spec.AcceptedCAs)
				suite.Assert().Equal("abc", spec.BootstrapTokenID)
				suite.Assert().Equal("def", spec.BootstrapTokenSecret)
				suite.Assert().Equal("", spec.EndpointTLSServerName)

				return nil
			},
		),
	)
}

// TestReconcileKubePrismTLSServerName verifies that when KubePrism is enabled
// with a tlsServerName, the kubelet endpoint stays on loopback and the
// EndpointTLSServerName is propagated to the resource for kubeconfig generation.
func (suite *KubeletSuite) TestReconcileKubePrismTLSServerName() {
	u, err := url.Parse("https://foo:6443")
	suite.Require().NoError(err)

	ca, err := x509.NewSelfSignedCertificateAuthority(x509.RSA(false))
	suite.Require().NoError(err)

	k8sCA := x509.NewCertificateAndKeyFromCertificateAuthority(ca)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineFeatures: &v1alpha1.FeaturesConfig{
						KubePrismSupport: &v1alpha1.KubePrism{
							ServerEnabled:       pointer.To(true),
							ServerPort:          7445,
							ServerTLSServerName: "cluster-xyz.example.com",
						},
					},
				},
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

	suite.Assert().NoError(
		retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			func() error {
				kubeletSecrets, err := ctest.Get[*secrets.Kubelet](
					suite,
					resource.NewMetadata(
						secrets.NamespaceName,
						secrets.KubeletType,
						secrets.KubeletID,
						resource.VersionUndefined,
					),
				)
				if err != nil {
					if state.IsNotFoundError(err) {
						return retry.ExpectedError(err)
					}

					return err
				}

				spec := kubeletSecrets.TypedSpec()

				if spec.EndpointTLSServerName != "cluster-xyz.example.com" {
					return retry.ExpectedErrorf("EndpointTLSServerName not propagated yet: %q", spec.EndpointTLSServerName)
				}

				suite.Assert().Equal("https://127.0.0.1:7445", spec.Endpoint.String())
				suite.Assert().Equal("cluster-xyz.example.com", spec.EndpointTLSServerName)

				return nil
			},
		),
	)
}
