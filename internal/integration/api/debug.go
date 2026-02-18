// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// DebugSuite ...
type DebugSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *DebugSuite) SuiteName() string {
	return "api.DebugSuite"
}

// SetupTest ...
func (suite *DebugSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)
}

// TearDownTest ...
func (suite *DebugSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestRunAlpine tests running a simple alpine container via DebugService.
func (suite *DebugSuite) TestRunAlpine() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	image := "docker.io/library/alpine:3.23"

	for _, driver := range []common.ContainerDriver{common.ContainerDriver_CRI, common.ContainerDriver_CONTAINERD} {
		rcv, err := suite.Client.ImageClient.Pull(ctx, &machine.ImageServicePullRequest{
			Containerd: &common.ContainerdInstance{
				Driver:    driver,
				Namespace: common.ContainerdNamespace_NS_SYSTEM,
			},
			ImageRef: image,
		})
		suite.Require().NoError(err)

		var pulledImage string

		for {
			msg, err := rcv.Recv()
			if err != nil {
				if errors.Is(err, io.EOF) {
					break
				}

				suite.Require().NoError(err)
			}

			// ignore progress messages, but the last message should contain the image name
			pulledImage = msg.GetName()
		}

		cli, err := suite.Client.DebugClient.ContainerRun(ctx)
		suite.Require().NoError(err)

		suite.Require().NoError(cli.Send(&machine.DebugContainerRunRequest{
			Request: &machine.DebugContainerRunRequest_Spec{
				Spec: &machine.DebugContainerRunRequestSpec{
					Containerd: &common.ContainerdInstance{
						Driver:    driver,
						Namespace: common.ContainerdNamespace_NS_SYSTEM,
					},
					ImageName: pulledImage,
					Profile:   machine.DebugContainerRunRequestSpec_PROFILE_PRIVILEGED,
					Tty:       true,
				},
			},
		}))

		readUntil := func(needle string) {
			var out strings.Builder

			for {
				msg, err := cli.Recv()
				suite.Require().NoError(err)

				if msg.GetStdoutData() != nil {
					out.Write(msg.GetStdoutData())
				}

				if strings.Contains(out.String(), needle) {
					return
				}
			}
		}

		readUntil("/ # ")

		suite.Require().NoError(cli.Send(&machine.DebugContainerRunRequest{
			Request: &machine.DebugContainerRunRequest_StdinData{
				StdinData: []byte("uname\n"),
			},
		}))

		readUntil("Linux")

		suite.Require().NoError(cli.CloseSend())

		for {
			msg, err := cli.Recv()
			suite.Require().NoError(err)

			if exitCode, ok := msg.GetResp().(*machine.DebugContainerRunResponse_ExitCode); ok {
				// SIGHUP is 128 + 1
				suite.Equal(int32(129), exitCode.ExitCode)

				break
			}

			suite.T().Logf("got extra stdout: %q", string(msg.GetStdoutData()))
		}
	}
}

func init() {
	allSuites = append(allSuites, new(DebugSuite))
}
