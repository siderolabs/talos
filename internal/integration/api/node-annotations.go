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
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// NodeAnnotationsSuite verifies updating node annotations via machine config.
type NodeAnnotationsSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *NodeAnnotationsSuite) SuiteName() string {
	return "api.NodeAnnotationsSuite"
}

// SetupTest ...
func (suite *NodeAnnotationsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *NodeAnnotationsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestUpdateControlPlane verifies node annotation updates on control plane nodes.
func (suite *NodeAnnotationsSuite) TestUpdateControlPlane() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.testUpdate(node)
}

// TestUpdateWorker verifies node annotation updates on worker nodes.
func (suite *NodeAnnotationsSuite) TestUpdateWorker() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	suite.testUpdate(node)
}

// testUpdate cycles through a set of node annotation updates reverting the change in the end.
func (suite *NodeAnnotationsSuite) testUpdate(node string) {
	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	suite.T().Logf("updating annotations on node %q (%q)", node, k8sNode.Name)

	watcher, err := suite.Clientset.CoreV1().Nodes().Watch(suite.ctx, metav1.ListOptions{
		FieldSelector: metadataKeyName + k8sNode.Name,
		Watch:         true,
	})
	suite.Require().NoError(err)

	defer watcher.Stop()

	// set two new annotation
	suite.setNodeAnnotations(node, map[string]string{
		"talos.dev/ann1": "value1",
		"talos.dev/ann2": "value2",
	})

	suite.waitUntil(watcher, map[string]string{
		"talos.dev/ann1": "value1",
		"talos.dev/ann2": "value2",
	})

	// remove one annotation owned by Talos
	suite.setNodeAnnotations(node, map[string]string{
		"talos.dev/ann1": "foo",
	})

	suite.waitUntil(watcher, map[string]string{
		"talos.dev/ann1": "foo",
		"talos.dev/ann2": "",
	})

	// remove all Talos annoations
	suite.setNodeAnnotations(node, nil)

	suite.waitUntil(watcher, map[string]string{
		"talos.dev/ann1": "",
		"talos.dev/ann2": "",
	})
}

func (suite *NodeAnnotationsSuite) waitUntil(watcher watch.Interface, expectedAnnotations map[string]string) {
outer:
	for {
		select {
		case ev := <-watcher.ResultChan():
			k8sNode, ok := ev.Object.(*v1.Node)
			suite.Require().Truef(ok, "watch event is not of type v1.Node but was %T", ev.Object)

			suite.T().Logf("annotations %#v", k8sNode.Annotations)

			for k, v := range expectedAnnotations {
				if v == "" {
					_, ok := k8sNode.Annotations[k]
					if ok {
						suite.T().Logf("annotation %q is still present", k)

						continue outer
					}
				}

				if k8sNode.Annotations[k] != v {
					suite.T().Logf("annotation %q is %q but expected %q", k, k8sNode.Annotations[k], v)

					continue outer
				}
			}

			return
		case <-suite.ctx.Done():
			suite.T().Fatal("timeout")
		}
	}
}

func (suite *NodeAnnotationsSuite) setNodeAnnotations(nodeIP string, nodeAnnotations map[string]string) { //nolint:dupl
	nodeCtx := client.WithNode(suite.ctx, nodeIP)

	nodeConfig, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	bytes := suite.PatchV1Alpha1Config(nodeConfig, func(nodeConfigRaw *v1alpha1.Config) {
		nodeConfigRaw.MachineConfig.MachineNodeAnnotations = nodeAnnotations
	})

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: bytes,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})

	suite.Require().NoError(err)
}

func init() {
	allSuites = append(allSuites, new(NodeAnnotationsSuite))
}
