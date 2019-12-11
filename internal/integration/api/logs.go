// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_api

package api

import (
	"context"
	"io"
	"io/ioutil"
	"time"

	"github.com/talos-systems/talos/api/common"
	"github.com/talos-systems/talos/cmd/osctl/pkg/client"
	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/constants"
)

// LogsSuite verifies Logs API
type LogsSuite struct {
	base.APISuite

	ctx       context.Context
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *LogsSuite) SuiteName() string {
	return "api.LogsSuite"
}

// SetupTest ...
func (suite *LogsSuite) SetupTest() {
	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 2*time.Minute)
}

// TearDownTest ...
func (suite *LogsSuite) TearDownTest() {
	suite.ctxCancel()
}

// TestServicesHaveLogs verifies that each service has logs
func (suite *LogsSuite) TestServicesHaveLogs() {
	servicesReply, err := suite.Client.ServiceList(suite.ctx)
	suite.Require().NoError(err)

	suite.Require().Len(servicesReply.Messages, 1)

	logsSize := int64(0)

	for _, svc := range servicesReply.Messages[0].Services {
		logsStream, err := suite.Client.Logs(
			suite.ctx,
			constants.SystemContainerdNamespace,
			common.ContainerDriver_CONTAINERD,
			svc.Id,
			false,
		)
		suite.Require().NoError(err)

		logReader, errCh, err := client.ReadStream(logsStream)
		suite.Require().NoError(err)

		n, err := io.Copy(ioutil.Discard, logReader)
		suite.Require().NoError(err)

		logsSize += n

		suite.Require().NoError(<-errCh)
	}

	// overall logs shouldn't be empty
	suite.Require().Greater(logsSize, int64(1024))
}

// TODO: TestContainersHaveLogs (CRI, containerd)

// TestServiceNotFound verifies error if service name is not found
func (suite *LogsSuite) TestServiceNotFound() {
	logsStream, err := suite.Client.Logs(
		suite.ctx,
		constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD,
		"nosuchservice",
		false,
	)
	suite.Require().NoError(err)

	suite.Require().NoError(logsStream.CloseSend())

	_, err = logsStream.Recv()
	suite.Require().Error(err)

	suite.Require().Regexp(`.+nosuchservice\.log: no such file or directory$`, err.Error())
}

// TestStreaming verifies that logs are streamed in real-time
func (suite *LogsSuite) TestStreaming() {
	logsStream, err := suite.Client.Logs(
		suite.ctx,
		constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD,
		"machined-api",
		true,
	)
	suite.Require().NoError(err)

	suite.Require().NoError(logsStream.CloseSend())

	respCh := make(chan *common.Data)
	errCh := make(chan error, 1)

	go func() {
		defer close(respCh)

		for {
			msg, err := logsStream.Recv()
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
		for range respCh {
		}
	}()

	// first, drain the stream until flow stops
DrainLoop:
	for {
		select {
		case msg, ok := <-respCh:
			suite.Require().True(ok)
			suite.Assert().NotEmpty(msg.Bytes)
		case <-time.After(200 * time.Millisecond):
			break DrainLoop
		}
	}

	// invoke machined API
	_, err = suite.Client.Version(suite.ctx)
	suite.Require().NoError(err)

	// there should be line in the logs
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
