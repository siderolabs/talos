// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"
	"testing"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// StatsSuite verifies dmesg command.
type StatsSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *StatsSuite) SuiteName() string {
	return "cli.StatsSuite"
}

// TestContainerd inspects stats via containerd driver.
func (suite *StatsSuite) TestContainerd() {
	suite.RunCLI(
		[]string{"stats", "--nodes", suite.RandomDiscoveredNodeInternalIP()},
		base.StdoutShouldMatch(regexp.MustCompile(`CPU`)),
		base.StdoutShouldMatch(regexp.MustCompile(`apid`)),
	)
}

// TestCRI inspects stats via CRI driver.
func (suite *StatsSuite) TestCRI() {
	suite.RunCLI(
		[]string{"stats", "--namespace", "cri", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`CPU`)),
		base.StdoutShouldMatch(regexp.MustCompile(`kube-system/kube-apiserver`)),
		base.StdoutShouldMatch(regexp.MustCompile(`k8s.io`)),
	)
}

// TestKubernetesFlagDeprecated covers the deprecated -k/--kubernetes alias: it still has to behave
// exactly like --namespace cri, and using it has to warn, not just silently keep working forever.
func (suite *StatsSuite) TestKubernetesFlagDeprecated() {
	suite.RunCLI(
		[]string{"stats", "-k", "--nodes", suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)},
		base.StdoutShouldMatch(regexp.MustCompile(`CPU`)),
		base.StdoutShouldMatch(regexp.MustCompile(`kube-system/kube-apiserver`)),
		base.StderrShouldMatch(regexp.MustCompile(`(?i)deprecated`)),
		base.StderrShouldMatch(regexp.MustCompile(`--namespace cri`)),
	)
}

// TestTalosContainers inspects stats for a container declared via a ContainerConfig document,
// addressed through --namespace taloscontainers.
func (suite *StatsSuite) TestTalosContainers() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Airgapped {
		suite.T().Skip("skipping test in airgapped mode, the test pulls an image")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	name := "talosctl-it-stats"

	cleanup := applyTalosContainer(&suite.CLISuite, node, name, images.DefaultSandboxImage, nil, nil)
	defer cleanup()

	suite.RunAndWaitForMatch(
		[]string{"stats", "--namespace", constants.TalosContainersContainerdNamespace, "--nodes", node},
		regexp.MustCompile(name),
		talosContainerStartTimeout,
	)
	suite.RunCLI(
		[]string{"stats", "--namespace", constants.TalosContainersContainerdNamespace, "--nodes", node},
		base.StdoutShouldMatch(regexp.MustCompile(`CPU`)),
	)
}

func init() {
	allSuites = append(allSuites, new(StatsSuite))
}
