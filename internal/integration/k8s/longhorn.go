// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_k8s

package k8s

import (
	"context"
	_ "embed"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

var (
	//go:embed testdata/longhorn-v2-engine-values.yaml
	longhornEngineV2Values []byte

	//go:embed testdata/longhorn-v2-storageclass.yaml
	longhornV2StorageClassManifest []byte

	//go:embed testdata/longhorn-v2-ublk-storageclass.yaml
	longhornV2UblkStorageClassManifest []byte

	//go:embed testdata/longhorn-v2-disk-patch.yaml
	longhornNodeDiskPatch []byte
)

// LongHornSuite tests deploying Longhorn with the v2 (SPDK) data engine.
//
// The v1 engine relies on exec'ing engine binaries the engine-image DaemonSet
// drops under /var/lib/longhorn/engine-binaries/, which is incompatible with
// noexec on /var (see LongHornV1Suite for the v1 path that opts out via the
// ephemeral-insecure VolumeConfig patch).
type LongHornSuite struct {
	base.K8sSuite
}

// SuiteName returns the name of the suite.
func (suite *LongHornSuite) SuiteName() string {
	return "k8s.LongHornSuite"
}

// TestDeploy tests deploying Longhorn (v2 data engine) and running fio against it.
func (suite *LongHornSuite) TestDeploy() {
	if suite.Cluster == nil {
		suite.T().Skip("without full cluster state reaching out to the node IP is not reliable")
	}

	if suite.CSITestName != "longhorn" {
		suite.T().Skip("skipping longhorn test as it is not enabled")
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
		longhornEngineV2Values,
	); err != nil {
		suite.T().Fatalf("failed to install Longhorn chart: %v", err)
	}

	longhornV2StorageClassUnstructured := suite.ParseManifests(longhornV2StorageClassManifest)
	longhornV2UblkStorageClassUnstructured := suite.ParseManifests(longhornV2UblkStorageClassManifest)

	suite.ApplyManifests(ctx, longhornV2StorageClassUnstructured)
	suite.ApplyManifests(ctx, longhornV2UblkStorageClassUnstructured)

	suite.T().Cleanup(func() {
		suite.DeleteManifests(ctx, longhornV2StorageClassUnstructured)
		suite.DeleteManifests(ctx, longhornV2UblkStorageClassUnstructured)
	})

	nodes := suite.DiscoverNodeInternalIPsByType(ctx, machine.TypeWorker)

	suite.Require().Equal(3, len(nodes), "expected 3 worker nodes")

	for _, node := range nodes {
		k8sNode, err := suite.GetK8sNodeByInternalIP(ctx, node)
		suite.Require().NoError(err)

		suite.Require().NoError(suite.WaitForResourceToBeAvailable(ctx, 2*time.Minute, "longhorn-system", "longhorn.io", "Node", "v1beta2", k8sNode.Name))

		suite.Require().NoError(suite.WaitForResource(ctx, "longhorn-system", "longhorn.io", "Node", "v1beta2", k8sNode.Name, "{.status.diskStatus.*.conditions[?(@.type==\"Ready\")].status}", "True"))
		suite.Require().NoError(suite.WaitForResource(ctx, "longhorn-system", "longhorn.io", "Node", "v1beta2", k8sNode.Name, "{.status.diskStatus.*.conditions[?(@.type==\"Schedulable\")].status}", "True"))

		suite.PatchK8sObject(ctx, "longhorn-system", "longhorn.io", "Node", "v1beta2", k8sNode.Name, longhornNodeDiskPatch)

		// Wait for the SPDK-managed nvme block disk to finish initializing
		// before running fio: replica scheduling on this disk is what fio-v2
		// exercises, and SPDK can take several seconds per node.
		suite.Require().NoError(suite.WaitForResource(
			ctx,
			"longhorn-system",
			"longhorn.io",
			"Node",
			"v1beta2",
			k8sNode.Name,
			"{.status.diskStatus.nvme.conditions[?(@.type==\"Ready\")].status}",
			"True",
		))
		suite.Require().NoError(suite.WaitForResource(
			ctx,
			"longhorn-system",
			"longhorn.io",
			"Node",
			"v1beta2",
			k8sNode.Name,
			"{.status.diskStatus.nvme.conditions[?(@.type==\"Schedulable\")].status}",
			"True",
		))
	}

	suite.Run("fio-v2", func() {
		suite.Require().NoError(suite.RunFIOTest(ctx, "longhorn-v2", "10G"))
	})

	suite.Run("fio-v2-ublk", func() {
		suite.Require().NoError(suite.RunFIOTest(ctx, "longhorn-v2-ublk", "10G"))
	})
}

func init() {
	allSuites = append(allSuites, new(LongHornSuite))
}
