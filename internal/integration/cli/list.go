// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
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
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	// Expected maximum number of separators in the output
	// In plain terms, it's the maximum depth of the directory tree
	maxSeps := 5

	if stdout, _ := suite.RunCLI(imageCacheQuery); strings.Contains(stdout, "ready") {
		// Image cache paths parts are longer
		maxSeps = 8
	}

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

			if !assert.LessOrEqual(
				t,
				actualSeparators,
				expectedSeparators,
				"too many separators, flags: %s\nlines:\n%s",
				strings.Join(flags, " "),
				stdout,
			) {
				return
			}

			maxActualSeparators = max(maxActualSeparators, actualSeparators)
		}

		assert.Equal(
			t,
			expectedSeparators,
			maxActualSeparators,
			"not enough separators, \nflags: %s\nlines:\n%s",
			strings.Join(flags, " "),
			stdout,
		)
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
		{separators: maxSeps, flags: []string{"--recurse=true"}},
	} {
		suite.Run(strings.Join(test.flags, ","), func() {
			runAndCheck(suite.T(), test.separators, test.flags...)
		})
	}
}

func init() {
	allSuites = append(allSuites, new(ListSuite))
}
