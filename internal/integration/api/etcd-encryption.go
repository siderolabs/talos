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

// TestEtcdEncryption verifies default and custom trusted CA roots.
func (suite *EtcdEncryptionSuite) TestEtcdEncryption() {
	// pick up a random node to test the EtcdEncryption on, and use it throughout the test
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.T().Logf("testing EtcdEncryption on node %s", node)

	// build a Talos API context which is tied to the node
	nodeCtx := client.WithNode(suite.ctx, node)

	cfgDocument := k8s.NewEtcdEncryptionConfigV1Alpha1()
	cfgDocument.Config = `
apiVersion: apiserver.config.k8s.io/v1
kind: EncryptionConfiguration
resources:
  - resources:
      - secrets
    providers:
      - aescbc:
        keys:
          - name: key1
            secret: c2VjcmV0IGlzIHNlY3VyZQ==
      - identity: {}
`

	// clean up custom config if it exists
	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

	// enable custom etcd encryption
	suite.PatchMachineConfig(nodeCtx, cfgDocument)

	suite.Require().Eventually(func() bool {
		return strings.Contains(suite.readEtcdEncryptionConfig(nodeCtx), "c2VjcmV0IGlzIHNlY3VyZQ==")
	}, 5*time.Second, 100*time.Millisecond)

	// deactivate the EtcdEncryption
	suite.RemoveMachineConfigDocuments(nodeCtx, cfgDocument.MetaKind)

	suite.Require().Eventually(func() bool {
		return !strings.Contains(suite.readEtcdEncryptionConfig(nodeCtx), "c2VjcmV0IGlzIHNlY3VyZQ==")
	}, 5*time.Second, 100*time.Millisecond)
}

func init() {
	allSuites = append(allSuites, new(EtcdEncryptionSuite))
}
