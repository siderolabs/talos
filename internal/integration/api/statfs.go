// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"path/filepath"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// StatfsSuite verifies the machine.StorageService Statfs API.
type StatfsSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *StatfsSuite) SuiteName() string {
	return "api.StatfsSuite"
}

// SetupTest ...
func (suite *StatfsSuite) SetupTest() {
	if !suite.Capabilities().SupportsVolumes {
		suite.T().Skip("cluster doesn't support volumes")
	}

	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), time.Second*10)
}

// TearDownTest ...
func (suite *StatfsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func (suite *StatfsSuite) statfs(nodeCtx context.Context, path string) (*machineapi.StorageServiceStatfsResponse, error) {
	return suite.Client.MachineStorageClient.Statfs(nodeCtx, &machineapi.StorageServiceStatfsRequest{
		Path: path,
	})
}

// TestStatfs verifies that the stats reported for a mounted filesystem are internally consistent.
func (suite *StatfsSuite) TestStatfs() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	info, err := suite.statfs(nodeCtx, constants.EphemeralMountPoint)
	suite.Require().NoError(err)

	suite.Assert().NotZero(info.GetSize(), "size should be non-zero")
	suite.Assert().LessOrEqual(info.GetUsed(), info.GetSize(), "used should not exceed size")

	// available space is what's left for unprivileged users, so it never accounts
	// for more than the unused part of the filesystem
	suite.Assert().LessOrEqual(info.GetUsed()+info.GetAvailable(), info.GetSize(),
		"used + available should not exceed size")

	suite.Assert().Equal(info.GetInodes(), info.GetInodesUsed()+info.GetInodesFree(),
		"used + free inodes should add up to the total")
}

// TestStatfsAnyPath verifies that the request takes any path, not just a mount point:
// a directory inside a filesystem reports that filesystem.
func (suite *StatfsSuite) TestStatfsAnyPath() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	mountPoint, err := suite.statfs(nodeCtx, constants.EphemeralMountPoint)
	suite.Require().NoError(err)

	// /var/lib directory lives on the ephemeral volume
	inside, err := suite.statfs(nodeCtx, filepath.Join(constants.EphemeralMountPoint, "lib"))
	suite.Require().NoError(err)

	suite.Assert().Equal(mountPoint.GetSize(), inside.GetSize())
	suite.Assert().Equal(mountPoint.GetInodes(), inside.GetInodes())
}

// TestStatfsNotFound verifies that a path which doesn't exist is an error rather than
// an empty result.
func (suite *StatfsSuite) TestStatfsNotFound() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	_, err := suite.statfs(nodeCtx, "/this/is/not/a/path")
	suite.Assert().Error(err)
}

func init() {
	allSuites = append(allSuites, new(StatfsSuite))
}
