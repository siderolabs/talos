// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package base

import (
	"fmt"
	"os/exec"
	"regexp"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/retry"
)

// CLISuite is a base suite for CLI tests
type CLISuite struct {
	suite.Suite
	TalosSuite
}

// DiscoverNodes provides list of Talos nodes in the cluster.
//
// As there's no way to provide this functionality via Talos CLI, it relies on cluster info.
func (cliSuite *CLISuite) DiscoverNodes() []string {
	discoveredNodes := cliSuite.TalosSuite.DiscoverNodes()
	if discoveredNodes != nil {
		return discoveredNodes
	}

	// still no nodes, skip the test
	cliSuite.T().Skip("no nodes were discovered")

	return nil
}

func (cliSuite *CLISuite) buildOsctlCmd(args []string) *exec.Cmd {
	// TODO: add support for calling `osctl config endpoint` before running osctl

	args = append([]string{"--talosconfig", cliSuite.TalosConfig}, args...)

	return exec.Command(cliSuite.OsctlPath, args...)
}

// RunOsctl runs osctl binary with the options provided
func (cliSuite *CLISuite) RunOsctl(args []string, options ...RunOption) {
	Run(&cliSuite.Suite, cliSuite.buildOsctlCmd(args), options...)
}

func (cliSuite *CLISuite) RunAndWaitForMatch(args []string, regex *regexp.Regexp, duration time.Duration, options ...retry.Option) {
	cliSuite.Assert().NoError(retry.Constant(duration, options...).Retry(func() error {
		stdout, _, err := RunAndWait(&cliSuite.Suite, cliSuite.buildOsctlCmd(args))
		if err != nil {
			return retry.UnexpectedError(err)
		}

		if !regex.MatchString(stdout.String()) {
			return retry.ExpectedError(fmt.Errorf("stdout doesn't match: %q", stdout))
		}

		return nil
	}))
}
