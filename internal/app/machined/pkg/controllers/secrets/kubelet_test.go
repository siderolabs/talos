// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package secrets_test

import (
	"net/url"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/go-retry/retry"

	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/secrets"
)

func TestKubeletSuite(t *testing.T) {
	suite.Run(t, &KubeletSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.KubeletController{}))
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
				suite.Assert().Equal(k8sCA, spec.CA)
				suite.Assert().Equal("abc", spec.BootstrapTokenID)
				suite.Assert().Equal("def", spec.BootstrapTokenSecret)

				return nil
			},
		),
	)
}
