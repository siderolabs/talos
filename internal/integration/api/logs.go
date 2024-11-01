// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// LogsSuite verifies Logs API.
type LogsSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc

	nodeCtx context.Context //nolint:containedctx
}

// SuiteName ...
func (suite *LogsSuite) SuiteName() string {
	return "api.LogsSuite"
}

// SetupTest ...
func (suite *LogsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)

	suite.nodeCtx = client.WithNodes(suite.ctx, suite.RandomDiscoveredNodeInternalIP())
}

// TearDownTest ...
func (suite *LogsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestServicesHaveLogs verifies that each service has logs.
func (suite *LogsSuite) TestServicesHaveLogs() {
	servicesReply, err := suite.Client.ServiceList(suite.nodeCtx)
	suite.Require().NoError(err)

	suite.Require().Len(servicesReply.Messages, 1)

	logsSize := int64(0)

	for _, svc := range servicesReply.Messages[0].Services {
		logsStream, err := suite.Client.Logs(
			suite.nodeCtx,
			constants.SystemContainerdNamespace,
			common.ContainerDriver_CONTAINERD,
			svc.Id,
			false,
			-1,
		)
		suite.Require().NoError(err)

		logReader, err := client.ReadStream(logsStream)
		suite.Require().NoError(err)

		n, err := io.Copy(io.Discard, logReader)
		suite.Require().NoError(err)

		logsSize += n
	}

	// overall logs shouldn't be empty
	suite.Require().Greater(logsSize, int64(1024))
}

// TestAuditdLogs verifies that auditd logs are present.
func (suite *LogsSuite) TestAuditdLogs() {
	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		suite.T().Skip("skip auditd logs test for non-QEMU clusters")
	}

	logsStream, err := suite.Client.Logs(
		suite.nodeCtx,
		constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD,
		"auditd",
		false,
		-1,
	)
	suite.Require().NoError(err)

	logReader, err := client.ReadStream(logsStream)
	suite.Require().NoError(err)

	n, err := io.Copy(io.Discard, logReader)
	suite.Require().NoError(err)

	// auditd logs shouldn't be empty
	suite.Require().Greater(n, int64(1024))
}

// TestTail verifies that log tail might be requested.
func (suite *LogsSuite) TestTail() {
	// invoke machined enough times to generate
	// some logs
	for range 20 {
		_, err := suite.Client.Version(suite.nodeCtx)
		suite.Require().NoError(err)
	}

	for _, tailLines := range []int32{0, 1, 10} {
		logsStream, err := suite.Client.Logs(
			suite.nodeCtx,
			constants.SystemContainerdNamespace,
			common.ContainerDriver_CONTAINERD,
			"apid",
			false,
			tailLines,
		)
		suite.Require().NoError(err)

		logReader, err := client.ReadStream(logsStream)
		suite.Require().NoError(err)

		scanner := bufio.NewScanner(logReader)
		lines := 0

		for scanner.Scan() {
			lines++
		}

		suite.Require().NoError(scanner.Err())

		suite.Assert().EqualValues(tailLines, lines)
	}
}

// TODO: TestContainersHaveLogs (CRI, containerd)

// TestServiceNotFound verifies error if service name is not found.
func (suite *LogsSuite) TestServiceNotFound() {
	logsStream, err := suite.Client.Logs(
		suite.nodeCtx,
		constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD,
		"nosuchservice",
		false,
		-1,
	)
	suite.Require().NoError(err)

	suite.Require().NoError(logsStream.CloseSend())

	msg, err := logsStream.Recv()
	suite.Require().NoError(err)

	suite.Require().Regexp(`.+log "nosuchservice" was not registered$`, msg.Metadata.Error)
}

// TestStreaming verifies that logs are streamed in real-time.
func (suite *LogsSuite) TestStreaming() {
	suite.testStreaming(-1)
}

// TestTailStreaming3 verifies tail + streaming.
func (suite *LogsSuite) TestTailStreaming3() {
	suite.testStreaming(3)
}

// TestTailStreaming0 verifies tail + streaming.
func (suite *LogsSuite) TestTailStreaming0() {
	suite.testStreaming(0)
}

//nolint:gocyclo
func (suite *LogsSuite) testStreaming(tailLines int32) {
	if tailLines >= 0 {
		// invoke machined enough times to generate
		// some logs
		for range tailLines {
			_, err := suite.Client.Stats(
				suite.nodeCtx,
				constants.SystemContainerdNamespace,
				common.ContainerDriver_CONTAINERD,
			)
			suite.Require().NoError(err)
		}
	}

	logsStream, err := suite.Client.Logs(
		suite.nodeCtx,
		constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD,
		"machined",
		true,
		tailLines,
	)
	suite.Require().NoError(err)

	suite.Require().NoError(logsStream.CloseSend())

	respCh := make(chan *common.Data)
	errCh := make(chan error, 1)

	go func() {
		defer close(respCh)

		for {
			msg, e := logsStream.Recv()
			if e != nil {
				errCh <- e

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

	linesDrained := 0

	// first, drain the stream until flow stops
DrainLoop:
	for {
		select {
		case msg, ok := <-respCh:
			suite.Require().True(ok)
			suite.Assert().NotEmpty(msg.Bytes)
			linesDrained += bytes.Count(msg.Bytes, []byte{'\n'})
		case <-time.After(200 * time.Millisecond):
			break DrainLoop
		}
	}

	suite.Assert().GreaterOrEqual(int32(linesDrained), tailLines)

	// invoke machined API
	_, err = suite.Client.Stats(suite.nodeCtx, constants.SystemContainerdNamespace, common.ContainerDriver_CONTAINERD)
	suite.Require().NoError(err)

	// there should be a line in the logs
	select {
	case msg, ok := <-respCh:
		suite.Require().True(ok)
		suite.Assert().NotEmpty(msg.Bytes)
	case <-time.After(200 * time.Millisecond):
		suite.Assert().Fail("no log message received")
	}

	select {
	case err = <-errCh:
		suite.Require().NoError(err)
	default:
	}
}

func init() {
	allSuites = append(allSuites, new(LogsSuite))
}
