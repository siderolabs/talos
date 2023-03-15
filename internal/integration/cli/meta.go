// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/siderolabs/talos/internal/integration/base"
)

// MetaSuite verifies meta sub-commands.
type MetaSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *MetaSuite) SuiteName() string {
	return "cli.MetaSuite"
}

// TestKey writes a META key, deletes it, verifies via resources.
func (suite *MetaSuite) TestKey() {
	node := suite.RandomDiscoveredNodeInternalIP()

	// detect docker platform and skip the test
	stdout, _ := suite.RunCLI([]string{"--nodes", node, "get", "platformmetadata"})
	if strings.Contains(stdout, "container") {
		suite.T().Skip("skipping on container platform")
	}

	key := "0x05" // unused/reserved key
	value := uuid.New().String()

	suite.RunCLI([]string{"--nodes", node, "meta", "write", key, value},
		base.StdoutEmpty())

	suite.RunCLI([]string{"--nodes", node, "get", "metakeys", key},
		base.StdoutShouldMatch(regexp.MustCompile(key)),
		base.StdoutShouldMatch(regexp.MustCompile(value)),
	)

	suite.RunCLI([]string{"--nodes", node, "meta", "delete", key},
		base.StdoutEmpty())

	suite.RunCLI([]string{"--nodes", node, "get", "metakeys", key},
		base.ShouldFail(),
		base.StderrShouldMatch(regexp.MustCompile("NotFound")),
	)
}

func init() {
	allSuites = append(allSuites, new(MetaSuite))
}
