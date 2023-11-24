// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"slices"
	"strings"
	"time"

	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/gen/xtesting/must"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// NodeTaintsSuite verifies updating node taints via machine config.
type NodeTaintsSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *NodeTaintsSuite) SuiteName() string {
	return "api.NodeTaintsSuite"
}

// SetupTest ...
func (suite *NodeTaintsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *NodeTaintsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestUpdateControlPlane verifies node taints updates on control plane nodes.
func (suite *NodeTaintsSuite) TestUpdateControlPlane() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	suite.testUpdate(node)
}

// testUpdate cycles through a set of node taints updates reverting the change in the end.
func (suite *NodeTaintsSuite) testUpdate(node string) {
	k8sNode, err := suite.GetK8sNodeByInternalIP(suite.ctx, node)
	suite.Require().NoError(err)

	suite.T().Logf("updating taints on node %q (%q)", node, k8sNode.Name)

	watcher, err := suite.Clientset.CoreV1().Nodes().Watch(suite.ctx, metav1.ListOptions{
		FieldSelector: metadataKeyName + k8sNode.Name,
		Watch:         true,
	})
	suite.Require().NoError(err)

	defer watcher.Stop()

	// set two new taints
	suite.setNodeTaints(node, map[string]string{
		"talos.dev/test1": "value1:NoSchedule",
		"talos.dev/test2": "NoSchedule",
	})

	suite.waitUntil(watcher, map[string]string{
		constants.LabelNodeRoleControlPlane: "NoSchedule",
		"talos.dev/test1":                   "value1:NoSchedule",
		"talos.dev/test2":                   "NoSchedule",
	})

	// remove one taint
	suite.setNodeTaints(node, map[string]string{
		"talos.dev/test1": "value1:NoSchedule",
	})

	suite.waitUntil(watcher, map[string]string{
		constants.LabelNodeRoleControlPlane: "NoSchedule",
		"talos.dev/test1":                   "value1:NoSchedule",
	})

	// remove all taints
	suite.setNodeTaints(node, nil)

	suite.waitUntil(watcher, map[string]string{
		constants.LabelNodeRoleControlPlane: "NoSchedule",
	})
}

func (suite *NodeTaintsSuite) waitUntil(watcher watch.Interface, expectedTaints map[string]string) {
outer:
	for {
		select {
		case ev := <-watcher.ResultChan():
			k8sNode, ok := ev.Object.(*v1.Node)
			suite.Require().Truef(ok, "watch event is not of type v1.Node but was %T", ev.Object)

			suite.T().Logf("labels %#v, taints %#v", k8sNode.Labels, k8sNode.Spec.Taints)

			taints := xslices.ToMap(k8sNode.Spec.Taints, func(t v1.Taint) (string, string) {
				switch {
				case t.Value == "":
					return t.Key, string(t.Effect)
				case t.Effect == "":
					return t.Key, t.Value
				default:
					return t.Key, strings.Join([]string{t.Value, string(t.Effect)}, ":")
				}
			})

			expectedTaintsKeys := maps.Keys(expectedTaints)

			slices.Sort(expectedTaintsKeys)

			for _, key := range expectedTaintsKeys {
				actualValue, ok := taints[key]
				if !ok {
					suite.T().Logf("taint %q is not present", key)

					continue outer
				}

				expectedValue := expectedTaints[key]

				if expectedValue != actualValue {
					suite.T().Logf("expected taint %q to be %q but was %q", key, expectedValue, actualValue)

					continue outer
				}

				delete(taints, key)
			}

			if len(taints) > 0 {
				keys := maps.Keys(taints)

				slices.Sort(keys)

				suite.T().Logf("taints %v are still present", keys)

				continue outer
			}

			return
		case <-suite.ctx.Done():
			suite.T().Fatal("timeout")
		}
	}
}

func (suite *NodeTaintsSuite) setNodeTaints(nodeIP string, nodeTaints map[string]string) {
	nodeCtx := client.WithNode(suite.ctx, nodeIP)

	nodeConfig := must.Value(suite.ReadConfigFromNode(nodeCtx))(suite.T())

	nodeConfigRaw := nodeConfig.RawV1Alpha1()
	suite.Require().NotNil(nodeConfigRaw, "node config is not of type v1alpha1.Config")

	nodeConfigRaw.MachineConfig.MachineNodeTaints = nodeTaints

	bytes := must.Value(container.NewV1Alpha1(nodeConfigRaw).Bytes())(suite.T())

	must.Value(suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: bytes,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	}))(suite.T())
}

func init() {
	allSuites = append(allSuites, new(NodeTaintsSuite))
}
