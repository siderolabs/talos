// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// EtcdEncryptionSuite ...
type EtcdEncryptionSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *EtcdEncryptionSuite) SuiteName() string {
	return "api.EtcdEncryptionSuite"
}

// SetupTest ...
func (suite *EtcdEncryptionSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 1*time.Minute)
}

// TearDownTest ...
func (suite *EtcdEncryptionSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func (suite *EtcdEncryptionSuite) readEtcdEncryptionConfig(nodeCtx context.Context) string {
	r, err := suite.Client.Read(nodeCtx, constants.KubernetesAPIServerSecretsDir+"/encryptionconfig.yaml")
	suite.Require().NoError(err)

	value, err := io.ReadAll(r)
	suite.Require().NoError(err)

	suite.Require().NoError(r.Close())

	return string(value)
}

// TestEtcdEncryption verifies that a custom etcd encryption config can be applied and reverted.
func (suite *EtcdEncryptionSuite) TestEtcdEncryption() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.T().Logf("testing EtcdEncryption on node %s", node)

	nodeCtx := client.WithNode(suite.ctx, node)

	cfgDocument := &k8s.EncryptionConfigurationDoc{
		Fields: map[string]any{
			"apiVersion": k8s.EncryptionConfigurationAPIVersion,
			"kind":       k8s.EncryptionConfigurationKind,
			"resources": []any{
				map[string]any{
					"resources": []any{"secrets"},
					"providers": []any{
						map[string]any{
							"aescbc": map[string]any{
								"keys": []any{
									map[string]any{
										"name":   "key1",
										"secret": "c2VjcmV0IGlzIHNlY3VyZQ==",
									},
								},
							},
						},
						map[string]any{
							"identity": map[string]any{},
						},
					},
				},
			},
		},
	}

	// clean up custom config if it exists
	suite.RemoveMachineConfigDocuments(nodeCtx, k8s.EncryptionConfigurationKind)

	// apply custom etcd encryption config
	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	suite.Require().Eventually(func() bool {
		return strings.Contains(suite.readEtcdEncryptionConfig(nodeCtx), "c2VjcmV0IGlzIHNlY3VyZQ==")
	}, 5*time.Second, 100*time.Millisecond)

	// apply a different encryption config and verify it fully replaces the previous one
	cfgDocument2 := &k8s.EncryptionConfigurationDoc{
		Fields: map[string]any{
			"apiVersion": k8s.EncryptionConfigurationAPIVersion,
			"kind":       k8s.EncryptionConfigurationKind,
			"resources": []any{
				map[string]any{
					"resources": []any{"secrets"},
					"providers": []any{
						map[string]any{
							"aesgcm": map[string]any{
								"keys": []any{
									map[string]any{
										"name":   "key2",
										"secret": "dGhpcyBpcyBhIG5ldyBrZXk=",
									},
								},
							},
						},
						map[string]any{
							"identity": map[string]any{},
						},
					},
				},
			},
		},
	}

	suite.PatchMachineConfig(nodeCtx, cfgDocument2)

	suite.Require().Eventually(func() bool {
		config := suite.readEtcdEncryptionConfig(nodeCtx)

		return strings.Contains(config, "dGhpcyBpcyBhIG5ldyBrZXk=") &&
			!strings.Contains(config, "c2VjcmV0IGlzIHNlY3VyZQ==") &&
			!strings.Contains(config, "aescbc")
	}, 5*time.Second, 100*time.Millisecond)

	// remove the custom encryption config
	suite.RemoveMachineConfigDocuments(nodeCtx, k8s.EncryptionConfigurationKind)

	suite.Require().Eventually(func() bool {
		config := suite.readEtcdEncryptionConfig(nodeCtx)

		return !strings.Contains(config, "dGhpcyBpcyBhIG5ldyBrZXk=") &&
			!strings.Contains(config, "c2VjcmV0IGlzIHNlY3VyZQ==")
	}, 5*time.Second, 100*time.Millisecond)
}

func init() {
	allSuites = append(allSuites, new(EtcdEncryptionSuite))
}
