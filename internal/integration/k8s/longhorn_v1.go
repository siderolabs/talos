// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"bytes"
	"context"
	_ "embed"
	"strings"
	"text/template"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

var (
	//go:embed testdata/longhorn-iscsi-volume.yaml
	longHornISCSIVolumeManifest []byte

	//go:embed testdata/longhorn-volumeattachment.yaml
	longHornISCSIVolumeAttachmentManifestTemplate []byte

	//go:embed testdata/pod-iscsi-volume.yaml
	podWithISCSIVolumeTemplate []byte
)

// LongHornV1Suite tests deploying Longhorn with the v1 data engine.
//
// The v1 engine's instance-manager exec's engine binaries it drops under
// /var/lib/longhorn/engine-binaries/, which only works when /var is mounted
// without noexec. The matching CI matrix entry applies
// hack/test/patches/ephemeral-insecure.yaml to disable Secure on the EPHEMERAL
// VolumeConfig.
type LongHornV1Suite struct {
	base.K8sSuite
}

// SuiteName returns the name of the suite.
func (suite *LongHornV1Suite) SuiteName() string {
	return "k8s.LongHornV1Suite"
}

// TestDeploy tests deploying Longhorn (v1 data engine) and running fio + an
// in-tree Kubernetes iscsi volume against a v1 iscsi-frontend Longhorn volume.
func (suite *LongHornV1Suite) TestDeploy() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	if suite.CSITestName != "longhorn-v1" {
		suite.T().Skip("skipping longhorn-v1 test as it is not enabled")
	}

	timeout, err := time.ParseDuration(suite.CSITestTimeout)
	if err != nil {
		suite.T().Fatalf("failed to parse timeout: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	suite.T().Cleanup(cancel)

	if err := suite.HelmInstall(
		ctx,
		"longhorn-system",
		"https://charts.longhorn.io",
		LongHornHelmChartVersion,
		"longhorn",
		"longhorn",
		nil,
	); err != nil {
		suite.T().Fatalf("failed to install Longhorn chart: %v", err)
	}

	nodes := suite.DiscoverNodeInternalIPsByType(ctx, machine.TypeWorker)

	suite.Require().Equal(3, len(nodes), "expected 3 worker nodes")

	for _, node := range nodes {
		k8sNode, err := suite.GetK8sNodeByInternalIP(ctx, node)
		suite.Require().NoError(err)

		suite.Require().NoError(suite.WaitForResourceToBeAvailable(ctx, 2*time.Minute, "longhorn-system", "longhorn.io", "Node", "v1beta2", k8sNode.Name))

		suite.Require().NoError(suite.WaitForResource(ctx, "longhorn-system", "longhorn.io", "Node", "v1beta2", k8sNode.Name, "{.status.diskStatus.*.conditions[?(@.type==\"Ready\")].status}", "True"))
		suite.Require().NoError(suite.WaitForResource(ctx, "longhorn-system", "longhorn.io", "Node", "v1beta2", k8sNode.Name, "{.status.diskStatus.*.conditions[?(@.type==\"Schedulable\")].status}", "True"))
	}

	suite.Run("fio", func() {
		suite.Require().NoError(suite.RunFIOTest(ctx, "longhorn", "10G"))
	})

	suite.Run("iscsi", func() {
		suite.testDeployISCSI(ctx)
	})
}

//nolint:gocyclo
func (suite *LongHornV1Suite) testDeployISCSI(ctx context.Context) {
	longHornISCSIVolumeManifestUnstructured := suite.ParseManifests(longHornISCSIVolumeManifest)

	defer func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, longHornISCSIVolumeManifestUnstructured)
	}()

	suite.ApplyManifests(ctx, longHornISCSIVolumeManifestUnstructured)

	tmpl, err := template.New("longhorn-iscsi-volumeattachment").Parse(string(longHornISCSIVolumeAttachmentManifestTemplate))
	suite.Require().NoError(err)

	var longHornISCSIVolumeAttachmentManifest bytes.Buffer

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	nodeInfo, err := suite.GetK8sNodeByInternalIP(ctx, node)
	if err != nil {
		suite.T().Fatalf("failed to get K8s node by internal IP: %v", err)
	}

	if err := tmpl.Execute(&longHornISCSIVolumeAttachmentManifest, struct {
		NodeID string
	}{
		NodeID: nodeInfo.Name,
	}); err != nil {
		suite.T().Fatalf("failed to render Longhorn ISCSI volume manifest: %v", err)
	}

	longHornISCSIVolumeAttachmentManifestUnstructured := suite.ParseManifests(longHornISCSIVolumeAttachmentManifest.Bytes())

	suite.ApplyManifests(ctx, longHornISCSIVolumeAttachmentManifestUnstructured)

	if err := suite.WaitForResource(ctx, "longhorn-system", "longhorn.io", "Volume", "v1beta2", "iscsi", "{.status.robustness}", "healthy"); err != nil {
		suite.T().Fatalf("failed to wait for LongHorn Engine to be Ready: %v", err)
	}

	if err := suite.WaitForResource(ctx, "longhorn-system", "longhorn.io", "Volume", "v1beta2", "iscsi", "{.status.state}", "attached"); err != nil {
		suite.T().Fatalf("failed to wait for LongHorn Engine to be Ready: %v", err)
	}

	if err := suite.WaitForResource(ctx, "longhorn-system", "longhorn.io", "Engine", "v1beta2", "iscsi-e-0", "{.status.currentState}", "running"); err != nil {
		suite.T().Fatalf("failed to wait for LongHorn Engine to be Ready: %v", err)
	}

	unstructured, err := suite.GetUnstructuredResource(ctx, "longhorn-system", "longhorn.io", "Engine", "v1beta2", "iscsi-e-0")
	if err != nil {
		suite.T().Fatalf("failed to get LongHorn Engine resource: %v", err)
	}

	var endpointData string

	if status, ok := unstructured.Object["status"].(map[string]any); ok {
		endpointData, ok = status["endpoint"].(string)
		if !ok {
			suite.T().Fatalf("failed to get LongHorn Engine endpoint")
		}
	}

	tmpl, err = template.New("pod-iscsi-volume").Parse(string(podWithISCSIVolumeTemplate))
	suite.Require().NoError(err)

	// endpoint is of the form `iscsi://10.244.0.5:3260/iqn.2019-10.io.longhorn:iscsi/1`
	// trim the iscsi:// prefix
	endpointData = strings.TrimPrefix(endpointData, "iscsi://")
	// trim the /1 suffix
	endpointData = strings.TrimSuffix(endpointData, "/1")

	targetPortal, IQN, ok := strings.Cut(endpointData, "/")
	if !ok {
		suite.T().Fatalf("failed to parse endpoint data from %s", endpointData)
	}

	var podWithISCSIVolume bytes.Buffer

	if err := tmpl.Execute(&podWithISCSIVolume, struct {
		NodeName     string
		TargetPortal string
		IQN          string
	}{
		NodeName:     nodeInfo.Name,
		TargetPortal: targetPortal,
		IQN:          IQN,
	}); err != nil {
		suite.T().Fatalf("failed to render pod with ISCSI volume manifest: %v", err)
	}

	podWithISCSIVolumeUnstructured := suite.ParseManifests(podWithISCSIVolume.Bytes())

	defer func() {
		cleanUpCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
		defer cleanupCancel()

		suite.DeleteManifests(cleanUpCtx, podWithISCSIVolumeUnstructured)
	}()

	suite.ApplyManifests(ctx, podWithISCSIVolumeUnstructured)

	suite.Require().NoError(suite.WaitForPodToBeRunning(ctx, 3*time.Minute, "default", "iscsipd"))
}

func init() {
	allSuites = append(allSuites, new(LongHornV1Suite))
}
