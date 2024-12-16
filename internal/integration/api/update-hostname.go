// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// UpdateHostnameSuite verifies UpdateHostname API.
type UpdateHostnameSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *UpdateHostnameSuite) SuiteName() string {
	return "api.UpdateHostnameSuite"
}

// SetupTest ...
func (suite *UpdateHostnameSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *UpdateHostnameSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestUpdateHostname updates the hostname of a worker node,
// then asserts that the node re-joins the cluster with the new hostname.
// It reverts the change at the end of the test and asserts that the node is reported again as Ready.
func (suite *UpdateHostnameSuite) TestUpdateHostname() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	nodeInternalIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	nodeCtx := client.WithNode(suite.ctx, nodeInternalIP)

	node, err := suite.GetK8sNodeByInternalIP(suite.ctx, nodeInternalIP)
	suite.Require().NoError(err)

	// ec2.internal and compute.internal are reserved domains in AWS
	// ref: https://docs.aws.amazon.com/AWSEC2/latest/UserGuide/ec2-instance-naming.html#instance-naming-ipbn
	if strings.HasSuffix(node.Name, ".ec2.internal") || strings.HasSuffix(node.Name, ".compute.internal") {
		suite.T().Skip("aws does not support hostname changes")
	}

	oldHostname := node.Name

	newHostname := "test-update-hostname"

	suite.T().Logf("updating hostname of node %q to %q (IP: %s)", oldHostname, newHostname, nodeInternalIP)

	err = suite.updateHostname(nodeCtx, newHostname)
	suite.Require().NoError(err)

	nodeReady := func(status corev1.ConditionStatus) bool {
		return status == corev1.ConditionTrue
	}

	nodeNotReady := func(status corev1.ConditionStatus) bool {
		return status != corev1.ConditionTrue
	}

	defer func() {
		suite.T().Logf("reverting hostname of node %q to %q (IP: %s)", newHostname, oldHostname, nodeInternalIP)

		// revert the hostname back to the original one
		err = suite.updateHostname(nodeCtx, oldHostname)
		suite.Require().NoError(err)

		suite.T().Logf("waiting for node %q to be ready", oldHostname)

		// expect node status to be Ready again
		suite.Assert().NoError(suite.WaitForK8sNodeReadinessStatus(suite.ctx, oldHostname, nodeReady))

		suite.T().Logf("deleting node %q", newHostname)

		// Delete the node with the test hostname
		err = suite.Clientset.CoreV1().Nodes().Delete(suite.ctx, newHostname, metav1.DeleteOptions{})
		suite.Require().NoError(err)

		suite.T().Logf("rebooting node %s", nodeInternalIP)

		// Reboot node for CNI bridge to be reconfigured: https://stackoverflow.com/questions/61373366
		suite.AssertRebooted(
			suite.ctx, nodeInternalIP, func(nodeCtx context.Context) error {
				return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
			}, 10*time.Minute,
			suite.CleanupFailedPods,
		)
	}()

	suite.T().Logf("waiting for node %q to be not ready", oldHostname)

	// expect node with old hostname to become NotReady
	suite.Assert().NoError(suite.WaitForK8sNodeReadinessStatus(suite.ctx, oldHostname, nodeNotReady))

	suite.T().Logf("waiting for node %q to be ready", newHostname)

	// expect node with new hostname to become Ready
	suite.Assert().NoError(suite.WaitForK8sNodeReadinessStatus(suite.ctx, newHostname, nodeReady))

	suite.T().Logf("deleting node %q", oldHostname)

	// Delete the node with the old hostname
	err = suite.Clientset.CoreV1().Nodes().Delete(suite.ctx, oldHostname, metav1.DeleteOptions{})
	suite.Require().NoError(err)
}

func (suite *UpdateHostnameSuite) updateHostname(nodeCtx context.Context, newHostname string) error {
	nodeConfig, err := suite.ReadConfigFromNode(nodeCtx)
	if err != nil {
		return err
	}

	bytes := suite.PatchV1Alpha1Config(nodeConfig, func(nodeConfigRaw *v1alpha1.Config) {
		nodeConfigRaw.MachineConfig.MachineNetwork.NetworkHostname = newHostname
	})

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: bytes,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})

	return err
}

func init() {
	allSuites = append(allSuites, new(UpdateHostnameSuite))
}
