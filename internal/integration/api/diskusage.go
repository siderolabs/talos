// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// DiskUsageSuite verifies Logs API.
type DiskUsageSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	nodeCtx context.Context //nolint:containedctx
}

// SuiteName ...
func (suite *DiskUsageSuite) SuiteName() string {
	return "api.DiskUsageSuite"
}

// SetupTest ...
func (suite *DiskUsageSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)

	suite.nodeCtx = client.WithNodes(suite.ctx, suite.RandomDiscoveredNodeInternalIP())
}

// TearDownTest ...
func (suite *DiskUsageSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestDiskUsageRequests compares results of disk usage requests with different parameters.
func (suite *DiskUsageSuite) TestDiskUsageRequests() {
	type testParams struct {
		recursionDepth int32
		all            bool
		paths          []string
	}

	defaultPaths := []string{
		"/etc",
		"/bin",
	}

	cases := []*testParams{
		{
			recursionDepth: 0,
			all:            false,
			paths:          defaultPaths,
		},
		{
			recursionDepth: 1,
			all:            false,
			paths:          defaultPaths,
		},
		{
			recursionDepth: 0,
			all:            true,
			paths:          defaultPaths,
		},
		{
			recursionDepth: 1,
			all:            true,
			paths:          defaultPaths,
		},
		{
			recursionDepth: 0,
			all:            true,
			paths:          append([]string{"/this/is/going/to/fail"}, defaultPaths...),
		},
	}

	sizes := map[string]int64{}

	for _, params := range cases {
		lookupPaths := map[string]bool{}
		for _, path := range params.paths {
			lookupPaths[path] = true
		}

		stream, err := suite.Client.DiskUsage(
			suite.nodeCtx,
			&machineapi.DiskUsageRequest{
				Paths:          params.paths,
				RecursionDepth: params.recursionDepth,
				All:            params.all,
			},
		)
		suite.Require().NoError(err)

		responseCount := 0

		for {
			info, err := stream.Recv()
			responseCount++

			if err != nil {
				if err == io.EOF || client.StatusCode(err) == codes.Canceled {
					break
				}

				suite.Require().NoError(err)
			}

			if size, ok := sizes[info.Name]; ok {
				suite.Require().EqualValues(size, info.Size)
			}

			sizes[info.Name] = info.Size
		}

		suite.Require().Greater(responseCount, 1)
	}
}

func init() {
	allSuites = append(allSuites, new(DiskUsageSuite))
}
