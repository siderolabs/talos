// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"encoding/json"
	"regexp"
	"testing"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/images"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// talosContainerStartTimeout covers a cold pull of a small, likely-cached image plus the controller
// chain that starts it.
const talosContainerStartTimeout = 3 * time.Minute

// applyTalosContainer declares a container via a ContainerConfig document on node, and returns a
// cleanup func that removes it again.
//
// entrypoint and args may be nil to use the image's own entrypoint. A distinct name per caller keeps
// the new namespace tests from clobbering each other if they ever run concurrently against the same
// node.
func applyTalosContainer(suite *base.CLISuite, node, name, image string, entrypoint, args []string) func() {
	patch := map[string]any{
		"apiVersion": "v1alpha1",
		"kind":       "ContainerConfig",
		"name":       name,
		"image":      image,
	}

	if len(entrypoint) > 0 {
		patch["entrypoint"] = entrypoint
	}

	if len(args) > 0 {
		patch["args"] = args
	}

	data, err := json.Marshal(patch)
	suite.Require().NoError(err)

	suite.RunCLI(
		[]string{"patch", "--nodes", node, "--patch", string(data), "machineconfig", "--mode=no-reboot"},
		base.StdoutEmpty(), base.StderrNotEmpty(),
	)

	return func() {
		removePatch := map[string]any{
			"apiVersion": "v1alpha1",
			"kind":       "ContainerConfig",
			"name":       name,
			"$patch":     "delete",
		}

		data, err := json.Marshal(removePatch)
		suite.Require().NoError(err)

		suite.RunCLI(
			[]string{"patch", "--nodes", node, "--patch", string(data), "machineconfig", "--mode=no-reboot"},
			base.StdoutEmpty(), base.StderrNotEmpty(),
		)
	}
}

// ContainersSuite verifies dmesg command.
type ContainersSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ContainersSuite) SuiteName() string {
	return "cli.ContainersSuite"
}

// TestContainerd inspects containers via containerd driver.
func (suite *ContainersSuite) TestContainerd() {
	suite.RunCLI(
		[]string{"containers", "--nodes", suite.RandomDiscoveredNodeInternalIP()},
		base.StdoutShouldMatch(regexp.MustCompile(`IMAGE`)),
		base.StdoutShouldMatch(regexp.MustCompile(`apid`)),
	)
}

// TestCRI inspects containers via CRI driver.
func (suite *ContainersSuite) TestCRI() {
	suite.RunCLI(
		[]string{
			"containers", "--namespace", "cri", "--nodes",
			suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane),
		},
		base.StdoutShouldMatch(regexp.MustCompile(`kube-system/kube-apiserver`)),
	)
}

// TestKubernetesFlagDeprecated covers the deprecated -k/--kubernetes alias: it still has to behave
// exactly like --namespace cri, and using it has to warn, not just silently keep working forever.
func (suite *ContainersSuite) TestKubernetesFlagDeprecated() {
	suite.RunCLI(
		[]string{
			"containers", "-k", "--nodes",
			suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane),
		},
		base.StdoutShouldMatch(regexp.MustCompile(`kube-system/kube-apiserver`)),
		base.StderrShouldMatch(regexp.MustCompile(`(?i)deprecated`)),
		base.StderrShouldMatch(regexp.MustCompile(`--namespace cri`)),
	)
}

// TestTalosContainers inspects a container declared via a ContainerConfig document, addressed through
// --namespace taloscontainers.
func (suite *ContainersSuite) TestTalosContainers() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Airgapped {
		suite.T().Skip("skipping test in airgapped mode, the test pulls an image")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	name := "talosctl-it-containers"

	cleanup := applyTalosContainer(&suite.CLISuite, node, name, images.DefaultSandboxImage, nil, nil)
	defer cleanup()

	suite.RunAndWaitForMatch(
		[]string{"containers", "--namespace", constants.TalosContainersContainerdNamespace, "--nodes", node},
		regexp.MustCompile(name),
		talosContainerStartTimeout,
	)
}

// TestNamespaceFlagsMutuallyExclusive verifies that --kubernetes and --namespace are refused together
// by the actual binary, end to end with the unit-level check in namespace_test.go which only exercises
// cobra's flag-group validation directly.
func (suite *ContainersSuite) TestNamespaceFlagsMutuallyExclusive() {
	suite.RunCLI(
		[]string{
			"containers", "--kubernetes", "--namespace", constants.TalosContainersContainerdNamespace,
			"--nodes", suite.RandomDiscoveredNodeInternalIP(),
		},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`kubernetes`)),
		base.StderrShouldMatch(regexp.MustCompile(`namespace`)),
	)
}

func init() {
	allSuites = append(allSuites, new(ContainersSuite))
}
