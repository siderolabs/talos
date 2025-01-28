// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	secretsres "github.com/siderolabs/talos/pkg/machinery/resources/secrets"
	"github.com/siderolabs/talos/pkg/provision/access"
	"github.com/siderolabs/talos/pkg/rotate/pki/kubernetes"
	"github.com/siderolabs/talos/pkg/rotate/pki/talos"
)

// RotateCASuite verifies rotation of Talos and Kubernetes CAs.
type RotateCASuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *RotateCASuite) SuiteName() string {
	return "api.RotateCASuite"
}

// SetupTest ...
func (suite *RotateCASuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Minute)
}

// TearDownTest ...
func (suite *RotateCASuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestTalos updates Talos CA in the cluster.
func (suite *RotateCASuite) TestTalos() {
	if suite.Cluster == nil {
		suite.T().Skip("cluster information is not available")
	}

	suite.T().Logf("capturing current Talos CA")

	nodeInternalIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	// save osRoot
	osRoot, err := safe.StateGetByID[*secretsres.OSRoot](client.WithNode(suite.ctx, nodeInternalIP), suite.Client.COSI, secretsres.OSRootID)
	suite.Require().NoError(err)

	suite.T().Logf("rotating current CA -> new CA")

	newBundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	suite.Require().NoError(err)

	options := talos.Options{
		CurrentClient: suite.Client,
		ClusterInfo:   access.NewAdapter(suite.Cluster),

		ContextName: suite.Talosconfig.Context,
		Endpoints:   suite.Client.GetEndpoints(),

		NewTalosCA: newBundle.Certs.OS,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}

	newTalosconfig, err := talos.Rotate(suite.ctx, options)
	suite.Require().NoError(err)

	newClient, err := client.New(suite.ctx, client.WithConfig(newTalosconfig))
	suite.Require().NoError(err)

	if !testing.Short() {
		suite.restartAPIServices(newClient)
	}

	suite.T().Logf("rotating back new CA -> old CA")

	options = talos.Options{
		CurrentClient: newClient,
		ClusterInfo:   access.NewAdapter(suite.Cluster),

		ContextName: suite.Talosconfig.Context,
		Endpoints:   suite.Client.GetEndpoints(),

		NewTalosCA: osRoot.TypedSpec().IssuingCA,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}

	_, err = talos.Rotate(suite.ctx, options)
	suite.Require().NoError(err)

	suite.AssertClusterHealthy(suite.ctx)

	suite.ClearConnectionRefused(suite.ctx, suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)...)
}

// TestKubernetes updates Kubernetes CA in the cluster.
func (suite *RotateCASuite) TestKubernetes() {
	if suite.Cluster == nil {
		suite.T().Skip("cluster information is not available")
	}

	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	suite.T().Logf("capturing current Kubernetes CA")

	nodeInternalIP := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	// save k8sRoot
	k8sRoot, err := safe.StateGetByID[*secretsres.KubernetesRoot](client.WithNode(suite.ctx, nodeInternalIP), suite.Client.COSI, secretsres.KubernetesRootID)
	suite.Require().NoError(err)

	suite.T().Logf("rotating current CA -> new CA")

	newBundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	suite.Require().NoError(err)

	options := kubernetes.Options{
		TalosClient: suite.Client,
		ClusterInfo: access.NewAdapter(suite.Cluster),

		NewKubernetesCA: newBundle.Certs.K8s,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}

	suite.Require().NoError(kubernetes.Rotate(suite.ctx, options))

	suite.AssertClusterHealthy(suite.ctx)

	suite.T().Logf("rotating back new CA -> old CA")

	options = kubernetes.Options{
		TalosClient: suite.Client,
		ClusterInfo: access.NewAdapter(suite.Cluster),

		NewKubernetesCA: k8sRoot.TypedSpec().IssuingCA,

		EncoderOption: encoder.WithComments(encoder.CommentsAll),

		Printf: suite.T().Logf,
	}

	suite.Require().NoError(kubernetes.Rotate(suite.ctx, options))

	suite.AssertClusterHealthy(suite.ctx)
}

func (suite *RotateCASuite) restartAPIServices(c *client.Client) {
	suite.T().Logf("restarting API services")

	var oldClient *client.Client
	oldClient, suite.Client = suite.Client, c

	defer func() {
		suite.Client = oldClient
	}()

	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane) {
		suite.T().Logf("restarting API services on %s", node)

		err := c.Restart(client.WithNode(suite.ctx, node), constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, "trustd")
		suite.Require().NoError(err)

		suite.ClearConnectionRefused(suite.ctx, node)

		err = c.Restart(client.WithNode(suite.ctx, node), constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, "apid")
		suite.Require().NoError(err)

		suite.ClearConnectionRefused(suite.ctx, node)
	}

	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker) {
		suite.T().Logf("restarting API services on %s", node)

		err := c.Restart(client.WithNode(suite.ctx, node), constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD, "apid")
		suite.Require().NoError(err)

		suite.ClearConnectionRefused(suite.ctx, node)
	}

	suite.AssertClusterHealthy(suite.ctx)
}

func init() {
	allSuites = append(allSuites, new(RotateCASuite))
}
