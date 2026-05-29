// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/client/multiplex"
)

// DmesgSuite verifies Dmesg API.
type DmesgSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *DmesgSuite) SuiteName() string {
	return "api.DmesgSuite"
}

// SetupTest ...
func (suite *DmesgSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)
}

// TearDownTest ...
func (suite *DmesgSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestNodeHasDmesg verifies that default node has dmesg.
func (suite *DmesgSuite) TestNodeHasDmesg() {
	dmesgStream, err := suite.Client.Dmesg(
		suite.ctx,
		false,
		false,
	)
	suite.Require().NoError(err)

	logReader, err := client.ReadStream(dmesgStream)
	suite.Require().NoError(err)

	n, err := io.Copy(io.Discard, logReader)
	suite.Require().NoError(err)

	// dmesg shouldn't be empty
	suite.Require().Greater(n, int64(1024))
}

// TestStreaming verifies that logs are streamed in real-time.
func (suite *DmesgSuite) TestStreaming() {
	dmesgStream, err := suite.Client.Dmesg(
		suite.ctx,
		true,
		false,
	)
	suite.Require().NoError(err)

	suite.Require().NoError(dmesgStream.CloseSend())

	respCh := make(chan *common.Data)
	errCh := make(chan error, 1)

	go func() {
		defer close(respCh)

		for {
			msg, err := dmesgStream.Recv()
			if err != nil {
				errCh <- err

				return
			}

			respCh <- msg
		}
	}()

	defer func() {
		suite.ctxCancel()
		// drain respCh
		for range respCh { //nolint:revive
		}
	}()

	// drain the stream until flow stops
	logCount := 0

DrainLoop:
	for {
		select {
		case msg, ok := <-respCh:
			logCount++

			suite.Require().True(ok)
			suite.Assert().NotEmpty(msg.Bytes)
		case <-time.After(200 * time.Millisecond):
			break DrainLoop
		}
	}

	suite.Assert().Greater(logCount, 10)
}

// TestClusterHasDmesg verifies that all the cluster nodes have dmesg.
func (suite *DmesgSuite) TestClusterHasDmesg() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)
	suite.Require().NotEmpty(nodes)

	respCh := multiplex.Streaming(suite.ctx, nodes, func(ctx context.Context) (machine.MachineService_DmesgClient, error) {
		return suite.Client.Dmesg(
			ctx,
			false,
			false,
		)
	})

	sizeByNode := map[string]int{}

	for resp := range respCh {
		suite.Require().NoError(resp.Err, "error calling Dmesg for node %q", resp.Node)

		sizeByNode[resp.Node] += len(resp.Payload.Bytes)
	}

	for _, node := range nodes {
		suite.Assert().Greater(sizeByNode[node], 1024)
	}

	for node := range sizeByNode {
		suite.Assert().Contains(nodes, node)
	}
}

func init() {
	allSuites = append(allSuites, new(DmesgSuite))
}
