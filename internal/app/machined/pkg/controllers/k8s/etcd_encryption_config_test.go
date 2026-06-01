// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

type EtcdEncryptionConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *EtcdEncryptionConfigSuite) TestLegacySecretboxOnly() {
	root := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	root.TypedSpec().SecretboxEncryptionSecret = "/FYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs="
	suite.Create(root)

	ctest.AssertResource(suite, k8s.EtcdEncryptionConfigID, func(res *k8s.EtcdEncryptionConfig, asrt *assert.Assertions) {
		asrt.Equal(
			`apiVersion: v1
kind: EncryptionConfig
resources:
- providers:
  - secretbox:
      keys:
      - name: key2
        secret: /FYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=
  - identity: {}
  resources:
  - secrets
`,
			res.TypedSpec().Configuration,
		)
	})
}

func (suite *EtcdEncryptionConfigSuite) TestLegacyAESCBCOnly() {
	root := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	root.TypedSpec().AESCBCEncryptionSecret = "/sFYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs="
	suite.Create(root)

	ctest.AssertResource(suite, k8s.EtcdEncryptionConfigID, func(res *k8s.EtcdEncryptionConfig, asrt *assert.Assertions) {
		asrt.Equal(
			`apiVersion: v1
kind: EncryptionConfig
resources:
- providers:
  - aescbc:
      keys:
      - name: key1
        secret: /sFYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=
  - identity: {}
  resources:
  - secrets
`,
			res.TypedSpec().Configuration,
		)
	})
}

func (suite *EtcdEncryptionConfigSuite) TestExplicitConfig() {
	root := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	root.TypedSpec().EtcdEncryptionConfig = map[string]any{
		"resources": []any{
			map[string]any{
				"resources": []string{"secrets"},
				"providers": []any{
					map[string]any{
						"secretbox": map[string]any{
							"keys": []any{
								map[string]any{
									"name":   "key2",
									"secret": "/FYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=",
								},
							},
						},
					},
					map[string]any{
						"aescbc": map[string]any{
							"keys": []any{
								map[string]any{
									"name":   "key1",
									"secret": "/sFYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=",
								},
							},
						},
					},
				},
			},
		},
	}
	suite.Create(root)

	ctest.AssertResource(suite, k8s.EtcdEncryptionConfigID, func(res *k8s.EtcdEncryptionConfig, asrt *assert.Assertions) {
		asrt.Equal(
			`apiVersion: v1
kind: EncryptionConfig
resources:
- providers:
  - secretbox:
      keys:
      - name: key2
        secret: /FYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=
  - aescbc:
      keys:
      - name: key1
        secret: /sFYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs=
  resources:
  - secrets
`,
			res.TypedSpec().Configuration,
		)
	})
}

func (suite *EtcdEncryptionConfigSuite) TestRemoveOnSecretsDestroy() {
	root := secrets.NewKubernetesRoot(secrets.KubernetesRootID)
	root.TypedSpec().SecretboxEncryptionSecret = "/FYehPLp5F8POCNQRVDEUb7Hmt+KkV44e+fQL4HMexs="
	suite.Create(root)

	ctest.AssertResource(suite, k8s.EtcdEncryptionConfigID, func(res *k8s.EtcdEncryptionConfig, asrt *assert.Assertions) {
		asrt.NotEmpty(res.TypedSpec().Configuration)
	})

	suite.Destroy(root)

	ctest.AssertNoResource[*k8s.EtcdEncryptionConfig](suite, k8s.EtcdEncryptionConfigID)
}

func TestEtcdEncryptionConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &EtcdEncryptionConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 10 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&k8sctrl.EtcdEncryptionConfigController{}))
			},
		},
	})
}
