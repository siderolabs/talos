// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// DiskFreeSuite verifies the diskfree (df) command.
//
// It embeds APISuite rather than CLISuite because provisioning a user volume to run
// against needs the machine config and block resource APIs.
type DiskFreeSuite struct {
	base.CLISuite
}

// SuiteName ...
func (suite *DiskFreeSuite) SuiteName() string {
	return "cli.DiskFreeSuite"
}

// SetupTest ...
func (suite *DiskFreeSuite) SetupTest() {
	if suite.Cluster != nil && suite.Cluster.Provisioner() == "docker" {
		suite.T().Skip("skip df command integration test as docker provisioner has no mounted disks")
	}
}

func findVolumeColumns(dataLines []string, volume string) []string {
	for _, line := range dataLines {
		columns := splitLine(line)

		// NODE VOLUME FILESYSTEM SIZE USED AVAILABLE USAGE MOUNT_POINT
		if len(columns) == 8 && columns[1] == volume {
			return columns
		}
	}

	return nil
}

// TestSuccess asserts that the command runs successfully.
func (suite *DiskFreeSuite) TestSuccess() {
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.RunCLI([]string{"diskfree", "--nodes", node},
		base.StdoutMatchFunc(func(stdout string) error {
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			if len(lines) < 2 {
				return errors.New("expected a header and at least one row")
			}

			for _, column := range []string{"VOLUME", "FILESYSTEM", "SIZE", "USED", "AVAILABLE", "USAGE", "MOUNT POINT"} {
				if !strings.Contains(lines[0], column) {
					return fmt.Errorf("missing column %q in header %q", column, lines[0])
				}
			}

			columns := findVolumeColumns(lines[1:], constants.EphemeralPartitionLabel)
			if columns == nil {
				return fmt.Errorf("EPHEMERAL volume not found in output:\n%s", stdout)
			}

			if mountedOn := columns[7]; mountedOn != constants.EphemeralMountPoint {
				return fmt.Errorf("expected EPHEMERAL mounted on %s, got %q", constants.EphemeralMountPoint, mountedOn)
			}

			if usage := columns[6]; !strings.HasSuffix(usage, "%") {
				return fmt.Errorf("expected USAGE as a percentage, got %q", usage)
			}

			return nil
		}))

	suite.RunCLI([]string{"diskfree", "--nodes", node, "--bytes"},
		base.StdoutMatchFunc(func(stdout string) error {
			lines := strings.Split(strings.TrimSpace(stdout), "\n")

			columns := findVolumeColumns(lines[1:], constants.EphemeralPartitionLabel)
			if columns == nil {
				return fmt.Errorf("EPHEMERAL volume not found in output:\n%s", stdout)
			}

			for _, idx := range []int{3, 4, 5} {
				if _, err := strconv.ParseUint(columns[idx], 10, 64); err != nil {
					return fmt.Errorf("expected integer in column %d, got %q: %w", idx, columns[idx], err)
				}
			}

			return nil
		}))
}

// TestMountPoint verifies that passing a mount point narrows the output to that one volume.
func (suite *DiskFreeSuite) TestMountPoint() {
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.RunCLI([]string{"diskfree", "--nodes", node, constants.EphemeralPartitionLabel},
		base.StdoutMatchFunc(func(stdout string) error {
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			if len(lines) != 2 {
				return fmt.Errorf("expected a header and exactly one row, got:\n%s", stdout)
			}

			columns := splitLine(lines[1])
			if columns[1] != constants.EphemeralPartitionLabel {
				return fmt.Errorf("expected the EPHEMERAL volume, got %q", lines[1])
			}

			if mountedOn := columns[7]; mountedOn != constants.EphemeralMountPoint {
				return fmt.Errorf("expected %s, got %q", constants.EphemeralMountPoint, mountedOn)
			}

			return nil
		}))
}

// TestInodes verifies the -i flag switches to the inode view.
func (suite *DiskFreeSuite) TestInodes() {
	node := suite.RandomDiscoveredNodeInternalIP()

	suite.RunCLI([]string{"diskfree", "--nodes", node, "-i"},
		base.StdoutMatchFunc(func(stdout string) error {
			lines := strings.Split(strings.TrimSpace(stdout), "\n")
			if len(lines) < 2 {
				return errors.New("expected a header and at least one row")
			}

			for _, column := range []string{"INODES", "IUSED", "IFREE", "%IUSED", "MOUNT POINT"} {
				if !strings.Contains(lines[0], column) {
					return fmt.Errorf("missing column %q in header %q", column, lines[0])
				}
			}

			return nil
		}))
}

// TestError runs the command with an error.
func (suite *DiskFreeSuite) TestError() {
	node := suite.RandomDiscoveredNodeInternalIP()

	// too many args
	suite.RunCLI(
		[]string{"diskfree", "--nodes", node, constants.EphemeralPartitionLabel, constants.StatePartitionLabel},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
	)

	// nothing mounted there
	suite.RunCLI(
		[]string{"diskfree", "--nodes", node, "NOT_A_VOLUME"},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
	)

	// the argument is a mount point, not an arbitrary path inside the filesystem
	suite.RunCLI(
		[]string{"diskfree", "--nodes", node, constants.CRIContainerdDataPath},
		base.ShouldFail(),
		base.StderrNotEmpty(),
		base.StdoutEmpty(),
	)
}

func init() {
	allSuites = append(allSuites, new(DiskFreeSuite))
}
