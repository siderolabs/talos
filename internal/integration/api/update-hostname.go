// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api
// +build integration_api

package api

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/talos-systems/talos/internal/integration/base"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/client"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
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

	nodeCtx := client.WithNodes(suite.ctx, nodeInternalIP)

	node, err := suite.GetK8sNodeByInternalIP(suite.ctx, nodeInternalIP)
	suite.Require().NoError(err)

	if strings.HasSuffix(node.Name, ".ec2.internal") {
		suite.T().Skip("aws does not support hostname changes")
	}

	oldHostname := node.Name

	newHostname := "test-update-hostname"

	err = suite.updateHostname(nodeCtx, newHostname)
	suite.Require().NoError(err)

	nodeReady := func(status corev1.ConditionStatus) bool {
		return status == corev1.ConditionTrue
	}

	nodeNotReady := func(status corev1.ConditionStatus) bool {
		return status != corev1.ConditionTrue
	}

	defer func() {
		// revert the hostname back to the original one
		err = suite.updateHostname(nodeCtx, oldHostname)
		suite.Require().NoError(err)

		// expect node status to be Ready again
		suite.Assert().NoError(suite.WaitForK8sNodeReadinessStatus(suite.ctx, oldHostname, nodeReady))

		// Delete the node with the test hostname
		err = suite.Clientset.CoreV1().Nodes().Delete(suite.ctx, newHostname, metav1.DeleteOptions{})
		suite.Require().NoError(err)

		// Reboot node for CNI bridge to be reconfigured: https://stackoverflow.com/questions/61373366
		suite.AssertRebooted(
			suite.ctx, nodeInternalIP, func(nodeCtx context.Context) error {
				return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
			}, 10*time.Minute,
		)
	}()

	// expect node with old hostname to become NotReady
	suite.Assert().NoError(suite.WaitForK8sNodeReadinessStatus(suite.ctx, oldHostname, nodeNotReady))

	// expect node with new hostname to become Ready
	suite.Assert().NoError(suite.WaitForK8sNodeReadinessStatus(suite.ctx, newHostname, nodeReady))

	// Delete the node with the old hostname
	err = suite.Clientset.CoreV1().Nodes().Delete(suite.ctx, oldHostname, metav1.DeleteOptions{})
	suite.Require().NoError(err)
}

func (suite *UpdateHostnameSuite) updateHostname(nodeCtx context.Context, newHostname string) error {
	nodeConfig, err := suite.ReadConfigFromNode(nodeCtx)
	if err != nil {
		return err
	}

	nodeConfigRaw, ok := nodeConfig.Raw().(*v1alpha1.Config)
	if !ok {
		return fmt.Errorf("unexpected node config type %T", nodeConfig.Raw())
	}

	nodeConfigRaw.MachineConfig.MachineNetwork.NetworkHostname = newHostname

	bytes, err := nodeConfigRaw.Bytes()
	if err != nil {
		return err
	}

	_, err = suite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: bytes,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})

	return err
}

func init() {
	allSuites = append(allSuites, new(UpdateHostnameSuite))
}
