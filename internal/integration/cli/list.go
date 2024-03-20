// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// ListSuite verifies dmesg command.
type ListSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *ListSuite) SuiteName() string {
	return "cli.ListSuite"
}

// TestSuccess runs comand with success.
func (suite *ListSuite) TestSuccess() {
	suite.RunCLI([]string{"list", "--nodes", suite.RandomDiscoveredNodeInternalIP(), "/etc"},
		base.StdoutShouldMatch(regexp.MustCompile(`os-release`)))

	suite.RunCLI([]string{"list", "--nodes", suite.RandomDiscoveredNodeInternalIP(), "/"},
		base.StdoutShouldNotMatch(regexp.MustCompile(`os-release`)))
}

// TestDepth tests various combinations of --recurse and --depth flags.
func (suite *ListSuite) TestDepth() {
	suite.T().Parallel()

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	// checks that enough separators are encountered in the output
	runAndCheck := func(t *testing.T, expectedSeparators int, flags ...string) {
		args := append([]string{"list", "--nodes", node, "/system"}, flags...)
		stdout, _ := suite.RunCLI(args)

		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		assert.Greater(t, len(lines), 2)
		assert.Equal(t, []string{"NODE", "NAME"}, strings.Fields(lines[0]))
		assert.Equal(t, []string{"."}, strings.Fields(lines[1])[1:])

		var maxActualSeparators int

		for _, line := range lines[2:] {
			actualSeparators := strings.Count(strings.Fields(line)[1], string(os.PathSeparator))

			msg := fmt.Sprintf(
				"too many separators (actualSeparators = %d, expectedSeparators = %d)\nflags: %s\nlines:\n%s",
				actualSeparators, expectedSeparators, strings.Join(flags, " "), strings.Join(lines, "\n"),
			)
			if !assert.LessOrEqual(t, actualSeparators, expectedSeparators, msg) {
				return
			}

			if maxActualSeparators < actualSeparators {
				maxActualSeparators = actualSeparators
			}
		}

		msg := fmt.Sprintf(
			"not enough separators (maxActualSeparators = %d, expectedSeparators = %d)\nflags: %s\nlines:\n%s",
			maxActualSeparators, expectedSeparators, strings.Join(flags, " "), strings.Join(lines, "\n"),
		)
		assert.Equal(t, maxActualSeparators, expectedSeparators, msg)
	}

	for _, test := range []struct {
		separators int
		flags      []string
	}{
		{separators: 0},

		{separators: 0, flags: []string{"--recurse=false"}},

		{separators: 0, flags: []string{"--depth=-1"}},
		{separators: 0, flags: []string{"--depth=0"}},
		{separators: 0, flags: []string{"--depth=1"}},
		{separators: 1, flags: []string{"--depth=2"}},
		{separators: 2, flags: []string{"--depth=3"}},

		{separators: 5, flags: []string{"--recurse=true"}},
	} {
		suite.Run(strings.Join(test.flags, ","), func() {
			suite.T().Parallel()
			runAndCheck(suite.T(), test.separators, test.flags...)
		})
	}
}

func init() {
	allSuites = append(allSuites, new(ListSuite))
}
