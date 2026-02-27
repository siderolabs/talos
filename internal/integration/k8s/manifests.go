// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/cluster/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/provision"
	"github.com/siderolabs/talos/pkg/provision/access"
)

// ManifestsSuite verifies Kubernetes manifest sync.
type ManifestsSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName returns the name of the suite.
func (suite *ManifestsSuite) SuiteName() string {
	return "k8s.ManifestsSuite"
}

// SetupTest ...
func (suite *ManifestsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)

	suite.ClearConnectionRefused(suite.ctx, suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeWorker)...)
	suite.AssertClusterHealthy(suite.ctx)
}

type manifestSyncWriter struct {
	t *testing.T
}

func (w manifestSyncWriter) Write(p []byte) (n int, err error) {
	w.t.Log("  " + string(p))

	return len(p), nil
}

// TestSync verifies that manifest sync works.
func (suite *ManifestsSuite) TestSync() {
	if suite.Cluster == nil {
		suite.T().Skip("skip without full cluster state")
	}

	cpNode := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	nodeCtx := client.WithNode(suite.ctx, cpNode)

	config, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	// some tests creates cluster without kube-proxy, skip in this case
	if !config.Cluster().Proxy().Enabled() {
		suite.T().Skip("skip when kube-proxy is disabled")
	}

	clusterAccess := access.NewAdapter(suite.Cluster, provision.WithTalosClient(suite.Client))
	defer clusterAccess.Close() //nolint:errcheck

	// 1. Patch all controlplane nodes with extra arg for kube-proxy
	suite.T().Log("adding extra arg to kube-proxy")

	extraArgPatch := map[string]any{
		"cluster": map[string]any{
			"proxy": map[string]any{
				"extraArgs": map[string]any{
					"nodeport-addresses": "0.0.0.0/0",
				},
			},
		},
	}

	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane) {
		suite.PatchMachineConfig(client.WithNode(suite.ctx, node), extraArgPatch)
	}

	// wait for the manifest to be updated
	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane) {
		rtestutils.AssertResource(client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
			"10-kube-proxy",
			func(manifest *k8s.Manifest, asrt *assert.Assertions) {
				marshaled, err := yaml.Marshal(manifest.TypedSpec())
				suite.Require().NoError(err)

				asrt.Contains(string(marshaled), "nodeport-addresses=0.0.0.0/0")
			},
		)
	}

	// 2. Roll out manifests
	suite.Require().NoError(kubernetes.PerformManifestsSync(
		suite.ctx,
		clusterAccess,
		true,
		kubernetes.UpgradeOptions{
			LogOutput:        manifestSyncWriter{t: suite.T()},
			ReconcileTimeout: 30 * time.Second,
		},
	))

	// 3. Assert that kube-proxy has the extra arg
	pods, err := suite.Clientset.CoreV1().Pods("kube-system").List(suite.ctx, metav1.ListOptions{
		LabelSelector: "k8s-app=kube-proxy",
	})
	suite.Require().NoError(err)
	suite.Require().NotEmpty(pods.Items)

	for _, pod := range pods.Items {
		suite.Require().NotEmpty(pod.Spec.Containers)

		suite.Assert().Contains(pod.Spec.Containers[0].Command, "--nodeport-addresses=0.0.0.0/0")
	}

	// 4. Disable kube-proxy
	suite.T().Log("disabling kube-proxy")

	disablePatch := map[string]any{
		"cluster": map[string]any{
			"proxy": map[string]any{
				"disabled": true,
			},
		},
	}

	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane) {
		suite.PatchMachineConfig(client.WithNode(suite.ctx, node), disablePatch)
	}

	// wait for the manifest to be removed
	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane) {
		rtestutils.AssertNoResource[*k8s.Manifest](client.WithNode(suite.ctx, node), suite.T(), suite.Client.COSI,
			"10-kube-proxy",
		)
	}

	// 5. Roll out manifests
	suite.Require().NoError(kubernetes.PerformManifestsSync(
		suite.ctx,
		clusterAccess,
		true,
		kubernetes.UpgradeOptions{
			LogOutput: manifestSyncWriter{t: suite.T()},
		},
	))

	// 6. Assert that kube-proxy is removed
	suite.Require().NoError(suite.EnsureResourceIsDeleted(suite.ctx, 10*time.Second, appsv1.SchemeGroupVersion.WithResource("daemonsets"), "kube-system", "kube-proxy"))

	// 7. Re-enable kube-proxy
	suite.T().Log("enabling kube-proxy")

	enablePatch := map[string]any{
		"cluster": map[string]any{
			"proxy": map[string]any{
				"disabled": map[string]any{
					"$patch": "delete",
				},
				"extraArgs": map[string]any{
					"$patch": "delete",
				},
			},
		},
	}

	for _, node := range suite.DiscoverNodeInternalIPsByType(suite.ctx, machine.TypeControlPlane) {
		suite.PatchMachineConfig(client.WithNode(suite.ctx, node), enablePatch)
	}

	// 8. Assert that kube-proxy is back
	suite.Require().NoError(
		suite.WaitForResourceToBeAvailable(suite.ctx, 30*time.Second, "kube-system", appsv1.GroupName, "DaemonSet", appsv1.SchemeGroupVersion.Version, "kube-proxy"),
	)

	suite.AssertClusterHealthy(suite.ctx)
}

func init() {
	allSuites = append(allSuites, new(ManifestsSuite))
}
