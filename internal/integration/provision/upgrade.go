// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/go-procfs/procfs"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	talosclient "github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

//nolint:maligned
type upgradeSpec struct {
	ShortName string

	InjectExtraKernelArgs *procfs.Cmdline

	SourceKernelPath     string
	SourceInitramfsPath  string
	SourceDiskImagePath  string
	SourceISOPath        string
	SourceInstallerImage string
	SourceVersion        string
	SourceK8sVersion     string

	TargetInstallerImage  string
	TargetVersion         string
	TargetK8sVersion      string
	TargetCmdlineContains string

	SkipKubeletUpgrade bool

	ControlplaneNodes int
	WorkerNodes       int

	// Deprecated: staged upgrades are not supported by the new LifecycleService API.
	// Use the legacy MachineService.Upgrade path instead.
	UpgradeStage            bool
	WithEncryption          bool
	WithTrustedBoot         bool
	WithBios                bool
	WithApplyConfig         bool
	WithSkipInjectingConfig bool
	WithEnforcing           bool
}

const (
	// These versions should be kept in sync with Makefile variable RELEASES.
	previousRelease = "v1.12.6"
	stableRelease   = "v1.13.0" // or soon-to-be-stable
	// The current version (the one being built on CI) is DefaultSettings.CurrentVersion.

	// Command to find Kubernetes version for past releases:
	//
	//  git show ${TAG}:pkg/machinery/constants/constants.go | grep KubernetesVersion
	previousK8sVersion = "1.35.2" // constants.DefaultKubernetesVersion in the previousRelease
	stableK8sVersion   = "1.36.0" // constants.DefaultKubernetesVersion in the stableRelease
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
			images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
			DefaultSettings.CurrentVersion,
		),
		TargetVersion:    DefaultSettings.CurrentVersion,
		TargetK8sVersion: currentK8sVersion,

		ControlplaneNodes: DefaultSettings.ControlplaneNodes,
		WorkerNodes:       DefaultSettings.WorkerNodes,

		WithEncryption: true,
	}
}

// upgradeCurrentToCurrent upgrades the current version to itself.
func upgradeCurrentToCurrent() upgradeSpec {
	installerImage := fmt.Sprintf(
		"%s/%s:%s",
		DefaultSettings.TargetInstallImageRegistry,
		images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
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
		images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
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
			images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
			DefaultSettings.CurrentVersion,
		),
		TargetVersion:    DefaultSettings.CurrentVersion,
		TargetK8sVersion: currentK8sVersion,

		ControlplaneNodes: 1,
		WorkerNodes:       0,
		UpgradeStage:      true,
	}
}

func upgradeCurrentToCurrentNewCmdline() upgradeSpec {
	installerImage := fmt.Sprintf(
		"%s/%s:%s",
		DefaultSettings.TargetInstallImageRegistry,
		images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
		DefaultSettings.CurrentVersion,
	)

	targetInstallerImage := installerImage + "-extra-cmdline"

	return upgradeSpec{
		ShortName: fmt.Sprintf("%s-same-ver-extra-cmdline", DefaultSettings.CurrentVersion),

		SourceISOPath:        helpers.ArtifactPath("metal-amd64.iso"),
		SourceInstallerImage: installerImage,
		SourceVersion:        DefaultSettings.CurrentVersion,
		SourceK8sVersion:     currentK8sVersion,

		TargetInstallerImage: targetInstallerImage,
		TargetVersion:        DefaultSettings.CurrentVersion,
		TargetK8sVersion:     currentK8sVersion,

		ControlplaneNodes: 1,
		WorkerNodes:       0,

		TargetCmdlineContains: "talos.extra_cmdline=extra-super-cmdline",

		WithApplyConfig: true,
	}
}

func upgradeCurrentToCurrentEnforcing() upgradeSpec {
	installerImage := fmt.Sprintf(
		"%s/%s:%s",
		DefaultSettings.TargetInstallImageRegistry,
		images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
		DefaultSettings.CurrentVersion,
	)

	return upgradeSpec{
		ShortName: fmt.Sprintf("%s-same-ver-enforcing", DefaultSettings.CurrentVersion),

		InjectExtraKernelArgs: procfs.NewCmdline("enforcing=1"),

		SourceISOPath:        helpers.ArtifactPath("metal-amd64.iso"),
		SourceInstallerImage: installerImage,
		SourceVersion:        DefaultSettings.CurrentVersion,
		SourceK8sVersion:     currentK8sVersion,

		TargetInstallerImage: installerImage,
		TargetVersion:        DefaultSettings.CurrentVersion,
		TargetK8sVersion:     currentK8sVersion,

		ControlplaneNodes: 1,
		WorkerNodes:       0,

		TargetCmdlineContains: "enforcing=1",

		WithApplyConfig: true,
		WithEnforcing:   true,
	}
}

// upgradeStableToCurrentTrustedBoot upgrades the stable Talos release to the current version
// with TPM-backed disk encryption (trustedboot). Both the source ISO and the source
// installer are produced from the upstream-tagged imager at the stable release, signed with
// the local secureboot/PCR keys; the target installer is the current-version secureboot
// installer (built locally) signed with the same keys, so the on-disk TPM token enrolled
// under the stable UKI can be unsealed under the current UKI after the upgrade.
//
// Pre-installed secureboot disk images with auto-enrolled SecureBoot keys only landed in
// the current development branch, not in 1.13.x — so the source uses ISO boot + maintenance
// install rather than a pre-installed disk image.
//
// Flow: ISO boots → maintenance mode → apply-config + install (pulls SourceInstallerImage,
// lays down stable UKI signed with local key) → reboot → stable Talos boots from disk →
// enrollment seals LUKS key under stable UKI's PCR pubkey (= local key K) → cluster
// healthy → talosctl upgrade replaces on-disk UKI with current (also signed with K) →
// reboot → current Talos unseals the blob enrolled by stable.
func upgradeStableToCurrentTrustedBoot() upgradeSpec {
	return upgradeSpec{
		// Short prefix is intentional: the cluster name becomes part of the swtpm
		// AF_UNIX socket path (~/.talos/clusters/<name>/<node>-tpm/swtpm.sock),
		// which has a 108-byte Linux limit. "trustedboot-v1.13.0-..." exceeds it.
		ShortName: fmt.Sprintf("tb-%s", DefaultSettings.CurrentVersion),

		SourceISOPath: helpers.ArtifactPath(filepath.Join(stableRelease, "metal-amd64-secureboot.iso")),
		// The stable secureboot installer is built locally with the test signing
		// keys. The `-stable-secureboot` tag distinguishes it from the real
		// <stable>-amd64-secureboot release image in the shared dev registry.
		// Must match the tag pushed by `make secureboot-stable-artifacts`.
		SourceInstallerImage: fmt.Sprintf(
			"%s/%s:%s-stable-secureboot",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
			stableRelease,
		),
		SourceVersion:    stableRelease,
		SourceK8sVersion: stableK8sVersion,

		TargetInstallerImage: fmt.Sprintf(
			"%s/%s:%s-amd64-secureboot",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName, //nolint:staticcheck // legacy is only used in tests
			DefaultSettings.CurrentVersion,
		),
		TargetVersion:    DefaultSettings.CurrentVersion,
		TargetK8sVersion: currentK8sVersion,

		ControlplaneNodes: 1,
		WorkerNodes:       0,

		WithEncryption:          true,
		WithTrustedBoot:         true,
		WithApplyConfig:         true,
		WithSkipInjectingConfig: true,
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

		InjectExtraKernelArgs: suite.spec.InjectExtraKernelArgs,

		SourceKernelPath:     suite.spec.SourceKernelPath,
		SourceInitramfsPath:  suite.spec.SourceInitramfsPath,
		SourceDiskImagePath:  suite.spec.SourceDiskImagePath,
		SourceISOPath:        suite.spec.SourceISOPath,
		SourceInstallerImage: suite.spec.SourceInstallerImage,
		SourceVersion:        suite.spec.SourceVersion,
		SourceK8sVersion:     suite.spec.SourceK8sVersion,

		WithEncryption:          suite.spec.WithEncryption,
		WithTrustedBoot:         suite.spec.WithTrustedBoot,
		WithBios:                suite.spec.WithBios,
		WithApplyConfig:         suite.spec.WithApplyConfig,
		WithSkipInjectingConfig: suite.spec.WithSkipInjectingConfig,
	})

	client, err := suite.clusterAccess.Client()
	suite.Require().NoError(err)

	// verify initial cluster version
	suite.assertSameVersionCluster(client, suite.spec.SourceVersion)

	// verify enforcing state
	for _, node := range suite.Cluster.Info().Nodes {
		rtestutils.AssertResource(
			talosclient.WithNode(suite.ctx, node.IPs[0].String()),
			suite.T(), client.COSI,
			runtime.SecurityStateID,
			func(r *runtime.SecurityState, asrt *assert.Assertions) {
				asrt.Equal(suite.spec.WithEnforcing, r.TypedSpec().SELinuxState == runtime.SELinuxStateEnforcing)
			},
		)
	}

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

	// verify enforcing state
	for _, node := range suite.Cluster.Info().Nodes {
		rtestutils.AssertResource(
			talosclient.WithNode(suite.ctx, node.IPs[0].String()),
			suite.T(), client.COSI,
			runtime.SecurityStateID,
			func(r *runtime.SecurityState, asrt *assert.Assertions) {
				asrt.Equal(suite.spec.WithEnforcing, r.TypedSpec().SELinuxState == runtime.SELinuxStateEnforcing)
			},
		)
	}

	// upgrade Kubernetes if required
	suite.upgradeKubernetes(suite.spec.SourceK8sVersion, suite.spec.TargetK8sVersion, suite.spec.SkipKubeletUpgrade)

	if suite.spec.TargetCmdlineContains != "" {
		for _, node := range suite.Cluster.Info().Nodes {
			suite.assertCmdlineContains(client, node.IPs[0].String(), suite.spec.TargetCmdlineContains)
		}
	}

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
		&UpgradeSuite{specGen: upgradeCurrentToCurrentNewCmdline, track: 2},
		&UpgradeSuite{specGen: upgradeCurrentToCurrentEnforcing, track: 1},
		&UpgradeSuite{specGen: upgradeStableToCurrentTrustedBoot, track: 0},
	)
}
