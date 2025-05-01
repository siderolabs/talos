// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"fmt"
	"path/filepath"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//nolint:maligned
type upgradeSpec struct {
	ShortName string

	SourceKernelPath     string
	SourceInitramfsPath  string
	SourceDiskImagePath  string
	SourceInstallerImage string
	SourceVersion        string
	SourceK8sVersion     string

	TargetInstallerImage string
	TargetVersion        string
	TargetK8sVersion     string

	SkipKubeletUpgrade bool

	ControlplaneNodes int
	WorkerNodes       int

	UpgradeStage    bool
	WithEncryption  bool
	WithBios        bool
	WithApplyConfig bool
}

const (
	// These versions should be kept in sync with Makefile variable RELEASES.
	previousRelease = "v1.9.5"
	stableRelease   = "v1.10.0" // or soon-to-be-stable
	// The current version (the one being built on CI) is DefaultSettings.CurrentVersion.

	// Command to find Kubernetes version for past releases:
	//
	//  git show ${TAG}:pkg/machinery/constants/constants.go | grep KubernetesVersion
	previousK8sVersion = "1.32.3" // constants.DefaultKubernetesVersion in the previousRelease
	stableK8sVersion   = "1.33.0" // constants.DefaultKubernetesVersion in the stableRelease
	currentK8sVersion  = constants.DefaultKubernetesVersion
)

// upgradePreviousToStable upgrades from the previous Talos release to the stable release.
func upgradePreviousToStable() upgradeSpec {
	return upgradeSpec{
		ShortName: fmt.Sprintf("%s-%s", previousRelease, stableRelease),

		SourceKernelPath: helpers.ArtifactPath(filepath.Join(trimVersion(previousRelease), constants.KernelAsset)),
		SourceInitramfsPath: helpers.ArtifactPath(
			filepath.Join(
				trimVersion(previousRelease),
				constants.InitramfsAsset,
			),
		),
		SourceInstallerImage: fmt.Sprintf("%s:%s", "ghcr.io/siderolabs/installer", previousRelease),
		SourceVersion:        previousRelease,
		SourceK8sVersion:     previousK8sVersion,

		TargetInstallerImage: fmt.Sprintf("%s:%s", "ghcr.io/siderolabs/installer", stableRelease),
		TargetVersion:        stableRelease,
		TargetK8sVersion:     stableK8sVersion,

		ControlplaneNodes: DefaultSettings.ControlplaneNodes,
		WorkerNodes:       DefaultSettings.WorkerNodes,
	}
}

// upgradeStableToCurrent upgrades from the stable Talos release to the current version.
func upgradeStableToCurrent() upgradeSpec {
	return upgradeSpec{
		ShortName: fmt.Sprintf("%s-%s", stableRelease, DefaultSettings.CurrentVersion),

		SourceKernelPath:     helpers.ArtifactPath(filepath.Join(trimVersion(stableRelease), constants.KernelAsset)),
		SourceInitramfsPath:  helpers.ArtifactPath(filepath.Join(trimVersion(stableRelease), constants.InitramfsAsset)),
		SourceInstallerImage: fmt.Sprintf("%s:%s", "ghcr.io/siderolabs/installer", stableRelease),
		SourceVersion:        stableRelease,
		SourceK8sVersion:     stableK8sVersion,

		TargetInstallerImage: fmt.Sprintf(
			"%s/%s:%s",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName,
			DefaultSettings.CurrentVersion,
		),
		TargetVersion:    DefaultSettings.CurrentVersion,
		TargetK8sVersion: currentK8sVersion,

		ControlplaneNodes: DefaultSettings.ControlplaneNodes,
		WorkerNodes:       DefaultSettings.WorkerNodes,
	}
}

// upgradeCurrentToCurrent upgrades the current version to itself.
func upgradeCurrentToCurrent() upgradeSpec {
	installerImage := fmt.Sprintf(
		"%s/%s:%s",
		DefaultSettings.TargetInstallImageRegistry,
		images.DefaultInstallerImageName,
		DefaultSettings.CurrentVersion,
	)

	return upgradeSpec{
		ShortName: fmt.Sprintf("%s-same-ver", DefaultSettings.CurrentVersion),

		SourceKernelPath:     helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath:  helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: installerImage,
		SourceVersion:        DefaultSettings.CurrentVersion,
		SourceK8sVersion:     currentK8sVersion,

		TargetInstallerImage: installerImage,
		TargetVersion:        DefaultSettings.CurrentVersion,
		TargetK8sVersion:     currentK8sVersion,

		ControlplaneNodes: DefaultSettings.ControlplaneNodes,
		WorkerNodes:       DefaultSettings.WorkerNodes,

		WithEncryption: true,
	}
}

// upgradeCurrentToCurrentBios upgrades the current version to itself without UEFI.
func upgradeCurrentToCurrentBios() upgradeSpec {
	installerImage := fmt.Sprintf(
		"%s/%s:%s",
		DefaultSettings.TargetInstallImageRegistry,
		images.DefaultInstallerImageName,
		DefaultSettings.CurrentVersion,
	)

	return upgradeSpec{
		ShortName: fmt.Sprintf("%s-same-ver-bios", DefaultSettings.CurrentVersion),

		SourceDiskImagePath:  helpers.ArtifactPath("metal-amd64.raw.zst"),
		SourceInstallerImage: installerImage,
		SourceVersion:        DefaultSettings.CurrentVersion,
		SourceK8sVersion:     currentK8sVersion,

		TargetInstallerImage: installerImage,
		TargetVersion:        DefaultSettings.CurrentVersion,
		TargetK8sVersion:     currentK8sVersion,

		ControlplaneNodes: DefaultSettings.ControlplaneNodes,
		WorkerNodes:       DefaultSettings.WorkerNodes,

		WithEncryption:  true,
		WithBios:        true,
		WithApplyConfig: true,
	}
}

// upgradeStableToCurrentPreserveStage upgrades from the stable Talos release to the current version for single-node cluster with preserve and stage.
func upgradeStableToCurrentPreserveStage() upgradeSpec {
	return upgradeSpec{
		ShortName: fmt.Sprintf("prsrv-stg-%s-%s", stableRelease, DefaultSettings.CurrentVersion),

		SourceKernelPath:     helpers.ArtifactPath(filepath.Join(trimVersion(stableRelease), constants.KernelAsset)),
		SourceInitramfsPath:  helpers.ArtifactPath(filepath.Join(trimVersion(stableRelease), constants.InitramfsAsset)),
		SourceInstallerImage: fmt.Sprintf("%s:%s", "ghcr.io/siderolabs/installer", stableRelease),
		SourceVersion:        stableRelease,
		SourceK8sVersion:     stableK8sVersion,

		TargetInstallerImage: fmt.Sprintf(
			"%s/%s:%s",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName,
			DefaultSettings.CurrentVersion,
		),
		TargetVersion:    DefaultSettings.CurrentVersion,
		TargetK8sVersion: currentK8sVersion,

		ControlplaneNodes: 1,
		WorkerNodes:       0,
		UpgradeStage:      true,
	}
}

// UpgradeSuite ...
type UpgradeSuite struct {
	BaseSuite

	specGen func() upgradeSpec
	spec    upgradeSpec

	track int
}

// SetupSuite ...
func (suite *UpgradeSuite) SetupSuite() {
	// call generate late in the flow, as it needs to pick up settings overridden by test runner
	suite.spec = suite.specGen()

	suite.T().Logf("upgrade spec = %v", suite.spec)

	suite.BaseSuite.SetupSuite()
}

// runE2E runs e2e test on the cluster.
func (suite *UpgradeSuite) runE2E(k8sVersion string) {
	if suite.spec.WorkerNodes == 0 {
		// no worker nodes, should make masters schedulable
		suite.untaint("control-plane-1")
	}

	suite.BaseSuite.runE2E(k8sVersion)
}

// TestRolling performs rolling upgrade starting with master nodes.
func (suite *UpgradeSuite) TestRolling() {
	suite.setupCluster(clusterOptions{
		ClusterName: suite.spec.ShortName,

		ControlplaneNodes: suite.spec.ControlplaneNodes,
		WorkerNodes:       suite.spec.WorkerNodes,

		SourceKernelPath:     suite.spec.SourceKernelPath,
		SourceInitramfsPath:  suite.spec.SourceInitramfsPath,
		SourceDiskImagePath:  suite.spec.SourceDiskImagePath,
		SourceInstallerImage: suite.spec.SourceInstallerImage,
		SourceVersion:        suite.spec.SourceVersion,
		SourceK8sVersion:     suite.spec.SourceK8sVersion,

		WithEncryption:  suite.spec.WithEncryption,
		WithBios:        suite.spec.WithBios,
		WithApplyConfig: suite.spec.WithApplyConfig,
	})

	client, err := suite.clusterAccess.Client()
	suite.Require().NoError(err)

	// verify initial cluster version
	suite.assertSameVersionCluster(client, suite.spec.SourceVersion)

	options := upgradeOptions{
		TargetInstallerImage: suite.spec.TargetInstallerImage,
		UpgradeStage:         suite.spec.UpgradeStage,
		TargetVersion:        suite.spec.TargetVersion,
	}

	// upgrade master nodes
	for _, node := range suite.Cluster.Info().Nodes {
		if node.Type == machine.TypeInit || node.Type == machine.TypeControlPlane {
			suite.upgradeNode(client, node, options)
		}
	}

	// upgrade worker nodes
	for _, node := range suite.Cluster.Info().Nodes {
		if node.Type == machine.TypeWorker {
			suite.upgradeNode(client, node, options)
		}
	}

	// verify final cluster version
	suite.assertSameVersionCluster(client, suite.spec.TargetVersion)

	// upgrade Kubernetes if required
	suite.upgradeKubernetes(suite.spec.SourceK8sVersion, suite.spec.TargetK8sVersion, suite.spec.SkipKubeletUpgrade)

	// run e2e test
	suite.runE2E(suite.spec.TargetK8sVersion)
}

// SuiteName ...
func (suite *UpgradeSuite) SuiteName() string {
	if suite.spec.ShortName == "" {
		suite.spec = suite.specGen()
	}

	return fmt.Sprintf("provision.UpgradeSuite.%s-TR%d", suite.spec.ShortName, suite.track)
}

func init() {
	allSuites = append(
		allSuites,
		&UpgradeSuite{specGen: upgradePreviousToStable, track: 0},
		&UpgradeSuite{specGen: upgradeStableToCurrent, track: 1},
		&UpgradeSuite{specGen: upgradeCurrentToCurrent, track: 2},
		&UpgradeSuite{specGen: upgradeCurrentToCurrentBios, track: 0},
		&UpgradeSuite{specGen: upgradeStableToCurrentPreserveStage, track: 1},
	)
}
