// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"maps"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/k8s"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// NodeLabelsSuite verifies updating node labels via machine config.
type NodeLabelsSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *NodeLabelsSuite) SuiteName() string {
	return "api.NodeLabelsSuite"
}

// SetupTest ...
func (suite *NodeLabelsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *NodeLabelsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestUpdateControlPlane verifies node label updates on control plane nodes.
func (suite *NodeLabelsSuite) TestUpdateControlPlane() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.testUpdate(node, true)
}

// TestUpdateWorker verifies node label updates on worker nodes.
func (suite *NodeLabelsSuite) TestUpdateWorker() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	suite.testUpdate(node, false)
}

// testUpdate cycles through a set of node label updates reverting the change in the end.
func (suite *NodeLabelsSuite) testUpdate(node string, isControlplane bool) {
	nodeConfig, err := suite.ReadConfigFromNode(client.WithNode(suite.ctx, node))
	suite.Require().NoError(err)

	if !nodeConfig.Has(k8s.KubeNodeConfig) {
		suite.T().Skipf("node %q does not have new-style KubeNodeConfig", node)
	}

	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	suite.T().Logf("updating labels on node %q (%q)", node, k8sNode.Name)

	watchCh := suite.SetupNodeInformer(suite.ctx, k8sNode.Name)

	const stdLabelName = "kubernetes.io/hostname"

	stdLabelValue := k8sNode.Labels[stdLabelName]

	// set two new labels
	suite.setNodeLabels(node, map[string]string{
		"talos.dev/test1": "value1",
		"talos.dev/test2": "value2",
	})

	suite.waitUntil(watchCh, map[string]string{
		"talos.dev/test1": "value1",
		"talos.dev/test2": "value2",
	})

	// remove one label owned by Talos
	suite.setNodeLabels(node, map[string]string{
		"talos.dev/test1": "foo",
	})

	suite.waitUntil(watchCh, map[string]string{
		"talos.dev/test1": "foo",
		"talos.dev/test2": "",
	})

	// on control plane node, try to override a label not owned by Talos
	if isControlplane {
		suite.setNodeLabels(node, map[string]string{
			"talos.dev/test1": "foo2",
			stdLabelName:      "bar",
		})

		suite.waitUntil(watchCh, map[string]string{
			"talos.dev/test1": "foo2",
			stdLabelName:      stdLabelValue,
		})
	}

	// remove all Talos Labels
	suite.setNodeLabels(node, nil)

	suite.waitUntil(watchCh, map[string]string{
		"talos.dev/test1": "",
		"talos.dev/test2": "",
	})
}

//nolint:gocyclo
func (suite *NodeLabelsSuite) waitUntil(watchCh <-chan *v1.Node, expectedLabels map[string]string) {
outer:
	for {
		select {
		case k8sNode := <-watchCh:
			suite.T().Logf("labels %#v, taints %#v", k8sNode.Labels, k8sNode.Spec.Taints)

			for k, v := range expectedLabels {
				if v == "" {
					_, ok := k8sNode.Labels[k]
					if ok {
						suite.T().Logf("label %q is still present", k)

						continue outer
					}
				}

				if k8sNode.Labels[k] != v {
					suite.T().Logf("label %q is %q but expected %q", k, k8sNode.Labels[k], v)

					continue outer
				}
			}

			return
		case <-suite.ctx.Done():
			suite.T().Fatal("timeout")
		}
	}
}

func (suite *NodeLabelsSuite) setNodeLabels(nodeIP string, nodeLabels map[string]string) { //nolint:dupl
	nodeCtx := client.WithNode(suite.ctx, nodeIP)

	nodeConfig, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	nodeLabels = maps.Clone(nodeLabels)

	nodeConfig, err = container.PatchDocument(
		nodeConfig,
		func(nodeConfig *k8s.KubeNodeConfigV1Alpha1) error {
			// preserve system labels
			for _, label := range []string{constants.LabelNodeRoleControlPlane, constants.LabelExcludeFromExternalLB} {
				if v, ok := nodeConfig.LabelsConfig[label]; ok {
					if nodeLabels == nil {
						nodeLabels = make(map[string]string)
					}

					nodeLabels[label] = v
				}
			}

			nodeConfig.LabelsConfig = nodeLabels

			return nil
		})
	suite.Require().NoError(err)

	bytes, err := nodeConfig.Bytes()
	suite.Require().NoError(err)

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: bytes,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})

	suite.Require().NoError(err)
}

func init() {
	allSuites = append(allSuites, new(NodeLabelsSuite))
}
