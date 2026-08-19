// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// talosContainerLogImage has a shell, which the pause image used for the other taloscontainers tests
// lacks, so it can be told to print something the test can look for in its logs.
const talosContainerLogImage = "docker.io/library/alpine:3.23"

// LogsSuite verifies logs command.
type LogsSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *LogsSuite) SuiteName() string {
	return "cli.LogsSuite"
}

// TestServiceLogs verifies that logs are displayed.
func (suite *LogsSuite) TestServiceLogs() {
	suite.RunCLI([]string{"logs", "kubelet", "--nodes", suite.RandomDiscoveredNodeInternalIP()}) // default checks for stdout not empty
}

// TestTailLogs verifies that logs can be displayed with tail lines.
func (suite *LogsSuite) TestTailLogs() {
	node := suite.RandomDiscoveredNodeInternalIP()

	// run some machined API calls to produce enough log lines
	for range 10 {
		suite.RunCLI([]string{"-n", node, "version"})
	}

	suite.RunCLI([]string{"logs", "apid", "-n", node, "--tail", "5"},
		base.StdoutMatchFunc(func(stdout string) error {
			lines := strings.Count(stdout, "\n")
			if lines != 5 {
				return fmt.Errorf("expected %d lines, found %d lines", 5, lines)
			}

			return nil
		}))
}

// TestServiceNotFound verifies that logs displays an error if service is not found.
func (suite *LogsSuite) TestServiceNotFound() {
	suite.RunCLI(
		[]string{"logs", "--nodes", suite.RandomDiscoveredNodeInternalIP(), "servicenotfound"},
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`error.+ log "servicenotfound" was not registered`)),
		base.ShouldFail(),
	)
}

// TestKubernetesFlagDeprecated covers the deprecated -k/--kubernetes alias reaching the CRI driver, and
// warning while it does.
//
// A nonexistent container id keeps this from depending on a specific pod being present, while still
// proving the request reached the CRI driver: the server's "not found" here is container-inspector
// text distinct from the ServiceLog "was not registered" text TestServiceNotFound checks for the
// system namespace, so a match confirms -k routed to the CRI path rather than falling back to system.
func (suite *LogsSuite) TestKubernetesFlagDeprecated() {
	suite.RunCLI(
		[]string{"logs", "-k", "--nodes", suite.RandomDiscoveredNodeInternalIP(), "talosctl-it-nonexistent"},
		base.StdoutEmpty(),
		base.ShouldFail(),
		base.StderrShouldMatch(regexp.MustCompile(`(?i)deprecated`)),
		base.StderrShouldMatch(regexp.MustCompile(`--namespace cri`)),
		base.StderrShouldMatch(regexp.MustCompile(`not found`)),
	)
}

// TestTalosContainerLogs verifies that logs for a container declared via a ContainerConfig document
// are readable through --namespace taloscontainers.
func (suite *LogsSuite) TestTalosContainerLogs() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Airgapped {
		suite.T().Skip("skipping test in airgapped mode, the test pulls an image")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	name := "talosctl-it-logs"
	marker := "talosctl-it-logs-marker"

	cleanup := applyTalosContainer(&suite.CLISuite, node, name, talosContainerLogImage,
		[]string{"/bin/sh", "-c"}, []string{"echo " + marker})
	defer cleanup()

	args := suite.MakeCMDFn([]string{
		"logs", "--namespace", constants.TalosContainersContainerdNamespace, "--nodes", node, name,
	})

	// unlike containers/stats, logs errors out until the container's log buffer exists, so a plain
	// command failure has to be retried too, not just a content mismatch.
	suite.Require().NoError(retry.Constant(talosContainerStartTimeout, retry.WithUnits(time.Second)).Retry(func() error {
		var stdout bytes.Buffer

		cmd := args()
		cmd.Stdout = &stdout

		if err := cmd.Run(); err != nil {
			return retry.ExpectedErrorf("logs command failed: %s", err)
		}

		if !regexp.MustCompile(marker).MatchString(stdout.String()) {
			return retry.ExpectedErrorf("stdout doesn't match %q: %q", marker, stdout.String())
		}

		return nil
	}))
}

func init() {
	allSuites = append(allSuites, new(LogsSuite))
}
