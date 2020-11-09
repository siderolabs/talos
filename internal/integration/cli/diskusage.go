// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/talos-systems/talos/internal/integration/base"
)

// DiskUsageSuite verifies dmesg command.
type DiskUsageSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *DiskUsageSuite) SuiteName() string {
	return "cli.DiskUsageSuite"
}

type duInfo struct {
	size int64
	name string
	node string
}

func splitLine(line string) []string {
	columns := []string{}

	parts := strings.Split(line, " ")
	for _, part := range parts {
		if part != "" {
			columns = append(columns, strings.TrimSpace(part))
		}
	}

	return columns
}

func parseLine(line string) (*duInfo, error) {
	columns := splitLine(line)

	if len(columns) < 2 || len(columns) > 3 {
		return nil, fmt.Errorf("failed to parse line %s", line)
	}

	res := &duInfo{}
	offset := 0

	if len(columns) == 3 {
		res.node = columns[0]
		offset++
	}

	size, err := strconv.ParseInt(columns[offset], 10, 64)
	if err != nil {
		return nil, err
	}

	res.size = size
	res.name = columns[offset+1]

	return res, nil
}

// TestSuccess runs comand with success.
func (suite *DiskUsageSuite) TestSuccess() {
	folder := "/var"
	node := suite.RandomDiscoveredNode()

	var folderSize int64 = 4096

	suite.RunCLI([]string{"list", "--nodes", node, folder, "-l"},
		base.StdoutMatchFunc(func(stdout string) error {
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			if len(lines) == 1 {
				return fmt.Errorf("expected lines > 0")
			}

			parts := splitLine(lines[1])
			var err error
			folderSize, err = strconv.ParseInt(parts[2], 10, 64)
			if err != nil {
				return err
			}

			return nil
		}))

	// check total calculation
	suite.RunCLI([]string{"usage", "--nodes", node, folder, "-d2", "--all"},
		base.StdoutMatchFunc(func(stdout string) error {
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			if len(lines) == 1 {
				return fmt.Errorf("expected lines > 0")
			}

			var totalExpected int64
			for _, line := range lines[1 : len(lines)-1] {
				info, err := parseLine(line)
				if err != nil {
					return err
				}

				totalExpected += info.size
			}

			// add folder size
			totalExpected += folderSize

			info, err := parseLine(lines[len(lines)-1])
			if err != nil {
				return err
			}
			if info.size != totalExpected {
				return fmt.Errorf("folder size was calculated incorrectly. Expected %d, got %d", totalExpected, info.size)
			}

			return nil
		}))
}

// TestError runs comand with error.
func (suite *DiskUsageSuite) TestError() {
	suite.RunCLI([]string{"usage", "--nodes", suite.RandomDiscoveredNode(), "/no/such/folder/here/just/for/sure"},
		base.StderrNotEmpty(), base.StdoutEmpty())
}

func init() {
	allSuites = append(allSuites, new(DiskUsageSuite))
}
