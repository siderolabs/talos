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
//
//nolint:gocyclo
func (apiSuite *APISuite) RunDebugContainer(ctx context.Context, node string, args ...string) (string, int32, error) {
	nodeCtx := client.WithNode(ctx, node)

	containerd := &common.ContainerdInstance{
		Driver:    common.ContainerDriver_CONTAINERD,
		Namespace: common.ContainerdNamespace_NS_SYSTEM,
	}

	// pull the image into the system namespace first
	rcv, err := apiSuite.Client.ImageClient.Pull(nodeCtx, &machineapi.ImageServicePullRequest{
		Containerd: containerd,
		ImageRef:   constants.DebugHostNsImage,
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to pull image %q: %w", constants.DebugHostNsImage, err)
	}

	var pulledImage string

	for {
		msg, err := rcv.Recv()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return "", 0, fmt.Errorf("failed to pull image %q: %w", constants.DebugHostNsImage, err)
		}

		pulledImage = msg.GetName()
	}

	cli, err := apiSuite.Client.DebugClient.ContainerRun(nodeCtx)
	if err != nil {
		return "", 0, fmt.Errorf("failed to start debug container: %w", err)
	}

	if err = cli.Send(&machineapi.DebugContainerRunRequest{
		Request: &machineapi.DebugContainerRunRequest_Spec{
			Spec: &machineapi.DebugContainerRunRequestSpec{
				Containerd: containerd,
				ImageName:  pulledImage,
				Args:       args,
				Profile:    machineapi.DebugContainerRunRequestSpec_PROFILE_HOST_NS,
			},
		},
	}); err != nil {
		return "", 0, fmt.Errorf("failed to send debug container spec: %w", err)
	}

	// no interactive input is sent, close the send side right away
	if err = cli.CloseSend(); err != nil {
		return "", 0, fmt.Errorf("failed to close debug container send stream: %w", err)
	}

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

			return out.String(), 0, fmt.Errorf("error receiving debug container output: %w", err)
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
		return out.String(), 0, fmt.Errorf("debug container stream closed without an exit code")
	}

	return out.String(), exitCode, nil
}
