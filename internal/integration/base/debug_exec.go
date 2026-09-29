// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package base

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// RunDebugContainer pulls the image (if needed) and runs a one-shot privileged debug container
// on the node via the DebugService, returning the combined stdout/stderr output and the exit code.
// Note: in non-TTY mode the server multiplexes stdout and stderr into a single stream, so the
// returned output contains both.
func (apiSuite *APISuite) RunDebugContainer(ctx context.Context, node string, args ...string) (string, int32) {
	nodeCtx := client.WithNode(ctx, node)

	containerd := &common.ContainerdInstance{
		Driver:    common.ContainerDriver_CONTAINERD,
		Namespace: common.ContainerdNamespace_NS_SYSTEM,
	}

	// pull the image into the system namespace first
	rcv, err := apiSuite.Client.ImageClient.Pull(nodeCtx, &machineapi.ImageServicePullRequest{
		Containerd: containerd,
		ImageRef:   constants.DebugNixyBoxImage,
	})
	apiSuite.Require().NoError(err, "failed to pull image %q", constants.DebugNixyBoxImage)

	var pulledImage string

	for {
		msg, err := rcv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			apiSuite.Require().NoError(err, "failed to pull image %q", constants.DebugNixyBoxImage)
		}

		pulledImage = msg.GetName()
	}

	apiSuite.Require().NotEmpty(pulledImage, "expected pulled image name in the response")

	cli, err := apiSuite.Client.DebugClient.ContainerRun(nodeCtx)
	apiSuite.Require().NoError(err, "failed to start debug container stream")

	err = cli.Send(&machineapi.DebugContainerRunRequest{
		Request: &machineapi.DebugContainerRunRequest_Spec{
			Spec: &machineapi.DebugContainerRunRequestSpec{
				Containerd: containerd,
				ImageName:  pulledImage,
				Args:       args,
				Profile:    machineapi.DebugContainerRunRequestSpec_PROFILE_HOST_NS,
			},
		},
	})
	apiSuite.Require().NoError(err, "failed to send debug container spec")

	// no interactive input is sent, close the send side right away
	err = cli.CloseSend()
	apiSuite.Require().NoError(err, "failed to close debug container send stream")

	var (
		out         strings.Builder
		exitCode    int32
		gotExitCode bool
	)

	// drain the stream until EOF, remembering the exit code: returning early on the exit code
	// message would leave the server-side stream hanging until the context is canceled.
	for {
		msg, err := cli.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			apiSuite.Require().NoError(err, "error receiving debug container output")
		}

		switch resp := msg.GetResp().(type) {
		case *machineapi.DebugContainerRunResponse_StdoutData:
			out.Write(resp.StdoutData)
		case *machineapi.DebugContainerRunResponse_ExitCode:
			exitCode = resp.ExitCode
			gotExitCode = true
		}
	}

	if !gotExitCode {
		apiSuite.Require().NoError(fmt.Errorf("debug container stream closed without an exit code"))
	}

	return out.String(), exitCode
}
