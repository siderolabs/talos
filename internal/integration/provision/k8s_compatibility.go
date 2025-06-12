// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_provision

package provision

import (
	"fmt"
	"slices"

	"github.com/blang/semver/v4"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/siderolabs/gen/maps"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/mgmt/helpers"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// K8sCompatibilitySuite ...
type K8sCompatibilitySuite struct {
	BaseSuite

	track int

	versionsSequence []string
}

// SuiteName ...
func (suite *K8sCompatibilitySuite) SuiteName() string {
	return fmt.Sprintf("provision.UpgradeSuite.KubernetesCompatibility-TR%d", suite.track)
}

// SetupSuite ...
func (suite *K8sCompatibilitySuite) SetupSuite() {
	// figure out Kubernetes versions to go through, the calculation is based on:
	//  * DefaultKubernetesVersion, e.g. 1.29.0
	//  * SupportedKubernetesVersions, e.g. 6
	//  * available `kubelet` images (tags)
	//
	// E.g. with example values above, upgrade will go through:
	// 1.24 -> 1.25 -> 1.26 -> 1.27 -> 1.28 -> 1.29 (6 versions)
	// For each past Kubernetes release, latest patch release will be used,
	// for the latest version (DefaultKubernetesVersion), the exact version will be used
	kubeletRepository, err := name.NewRepository(constants.KubeletImage)
	suite.Require().NoError(err)

	maxVersion, err := semver.Parse(constants.DefaultKubernetesVersion)
	suite.Require().NoError(err)

	minVersion := semver.Version{
		Major: maxVersion.Major,
		Minor: maxVersion.Minor - constants.SupportedKubernetesVersions + 1,
		Patch: 0,
	}

	// while Talos is in alpha stage, DefaultKubernetesVersion might be 1 minor behind the latest alpha Kubernetes version,
	// so we need to ensure that minVersion fits into compatibility range
	minVersionAdjusted := false

	currentTalosVersion, err := compatibility.ParseTalosVersion(version.NewVersion())
	suite.Require().NoError(err)

	minKubernetesVersion, err := compatibility.ParseKubernetesVersion(minVersion.String())
	suite.Require().NoError(err)

	if minKubernetesVersion.SupportedWith(currentTalosVersion) != nil {
		// bump up minVersion to the next minor version
		minVersion.Minor++
		minVersionAdjusted = true
	}

	type versionInfo struct {
		Major uint64
		Minor uint64
	}

	versionsToUse := map[versionInfo]semver.Version{
		{
			Major: maxVersion.Major,
			Minor: maxVersion.Minor,
		}: maxVersion,
	}

	tags, err := remote.List(kubeletRepository)
	suite.Require().NoError(err)

	for _, tag := range tags {
		version, err := semver.ParseTolerant(tag)
		if err != nil {
			continue
		}

		if version.Pre != nil {
			continue
		}

		if version.LT(minVersion) {
			continue
		}

		if version.GT(maxVersion) {
			continue
		}

		versionKey := versionInfo{
			Major: version.Major,
			Minor: version.Minor,
		}

		if curVersion := versionsToUse[versionKey]; version.GT(curVersion) {
			versionsToUse[versionKey] = version
		}
	}

	k8sVersions := maps.Values(versionsToUse)

	slices.SortFunc(k8sVersions, func(a, b semver.Version) int {
		return a.Compare(b)
	})

	suite.versionsSequence = xslices.Map(k8sVersions, semver.Version.String)

	suite.T().Logf("using following upgrade sequence: %v", suite.versionsSequence)

	if minVersionAdjusted {
		suite.T().Logf("min Kubernetes version was adjusted to %s to fit Talos compatibility range", minVersion.String())
		suite.Assert().Len(suite.versionsSequence, constants.SupportedKubernetesVersions-1)
	} else {
		suite.Assert().Len(suite.versionsSequence, constants.SupportedKubernetesVersions)
	}

	suite.BaseSuite.SetupSuite()
}

// TestAllVersions tries to run cluster on all Kubernetes versions.
func (suite *K8sCompatibilitySuite) TestAllVersions() {
	// start a cluster using latest Talos, and on earliest supported Kubernetes version
	suite.setupCluster(clusterOptions{
		ClusterName: "k8s-compat",

		ControlplaneNodes: DefaultSettings.ControlplaneNodes,
		WorkerNodes:       DefaultSettings.WorkerNodes,

		SourceKernelPath:    helpers.ArtifactPath(constants.KernelAssetWithArch),
		SourceInitramfsPath: helpers.ArtifactPath(constants.InitramfsAssetWithArch),
		SourceInstallerImage: fmt.Sprintf(
			"%s/%s:%s",
			DefaultSettings.TargetInstallImageRegistry,
			images.DefaultInstallerImageName,
			DefaultSettings.CurrentVersion,
		),
		SourceVersion:    DefaultSettings.CurrentVersion,
		SourceK8sVersion: suite.versionsSequence[0],
	})

	suite.runE2E(suite.versionsSequence[0])

	// for each next supported Kubernetes version, upgrade k8s and run e2e tests
	for i := 1; i < len(suite.versionsSequence); i++ {
		suite.upgradeKubernetes(suite.versionsSequence[i-1], suite.versionsSequence[i], false)

		suite.waitForClusterHealth()

		suite.runE2E(suite.versionsSequence[i])
	}
}

func init() {
	allSuites = append(
		allSuites,
		&K8sCompatibilitySuite{track: 2},
	)
}
