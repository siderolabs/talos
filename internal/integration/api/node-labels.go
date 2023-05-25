// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
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
	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	suite.T().Logf("updating labels on node %q (%q)", node, k8sNode.Name)

	watcher, err := suite.Clientset.CoreV1().Nodes().Watch(suite.ctx, metav1.ListOptions{
		FieldSelector: "metadata.name=" + k8sNode.Name,
		Watch:         true,
	})
	suite.Require().NoError(err)

	defer watcher.Stop()

	const stdLabelName = "kubernetes.io/hostname"

	stdLabelValue := k8sNode.Labels[stdLabelName]

	// set two new labels
	suite.setNodeLabels(node, map[string]string{
		"talos.dev/test1": "value1",
		"talos.dev/test2": "value2",
	})

	suite.waitUntil(watcher, map[string]string{
		"talos.dev/test1": "value1",
		"talos.dev/test2": "value2",
	})

	// remove one label owned by Talos
	suite.setNodeLabels(node, map[string]string{
		"talos.dev/test1": "foo",
	})

	suite.waitUntil(watcher, map[string]string{
		"talos.dev/test1": "foo",
		"talos.dev/test2": "",
	})

	// on control plane node, try to override a label not owned by Talos
	if isControlplane {
		suite.setNodeLabels(node, map[string]string{
			"talos.dev/test1": "foo2",
			stdLabelName:      "bar",
		})

		suite.waitUntil(watcher, map[string]string{
			"talos.dev/test1": "foo2",
			stdLabelName:      stdLabelValue,
		})
	}

	// remove all Talos Labels
	suite.setNodeLabels(node, nil)

	suite.waitUntil(watcher, map[string]string{
		"talos.dev/test1": "",
		"talos.dev/test2": "",
	})
}

func (suite *NodeLabelsSuite) waitUntil(watcher watch.Interface, expectedLabels map[string]string) {
outer:
	for {
		select {
		case ev := <-watcher.ResultChan():
			k8sNode, ok := ev.Object.(*v1.Node)
			suite.Require().True(ok, "watch event is not of type v1.Node")

			suite.T().Logf("labels %v", k8sNode.Labels)

			for k, v := range expectedLabels {
				if v == "" {
					_, ok := k8sNode.Labels[k]
					if ok {
						suite.T().Logf("label %q is still present", k)

						continue outer
					}
				}

				if k8sNode.Labels[k] != v {
					suite.T().Logf("label %q is not %q", k, v)

					continue outer
				}
			}

			return
		case <-suite.ctx.Done():
			suite.T().Fatal("timeout")
		}
	}
}

func (suite *NodeLabelsSuite) setNodeLabels(nodeIP string, nodeLabels map[string]string) {
	nodeCtx := client.WithNode(suite.ctx, nodeIP)

	nodeConfig, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	nodeConfigRaw := nodeConfig.RawV1Alpha1()
	suite.Require().NotNil(nodeConfigRaw, "node config is not of type v1alpha1.Config")

	nodeConfigRaw.MachineConfig.MachineNodeLabels = nodeLabels

	bytes, err := container.NewV1Alpha1(nodeConfigRaw).Bytes()
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
