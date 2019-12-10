// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runner

import (
	"context"
	"errors"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"

	"github.com/talos-systems/talos/pkg/retry"
)

// ContainerConfigs hold the configs we use to launch our container
type ContainerConfigs struct {
	ContainerConfig *container.Config
	HostConfig      *container.HostConfig
}

// CommandLocal runs a local binary. Used for osctl cluster create and setup
func CommandLocal(command string) error {
	commandSplit := strings.Split(command, " ")
	log.Println("issuing local command : '" + command + "'")

	_, err := exec.Command(commandSplit[0], commandSplit[1:]...).Output()
	if err != nil {
		return err
	}

	return nil
}

// CommandInContainer simply runs a bash command in the hyperkube containers
// nolint: gocyclo
func CommandInContainer(ctx context.Context, client *client.Client, runnerConfig *ContainerConfigs, command string) error {
	runnerConfig.ContainerConfig.Entrypoint = []string{"/bin/bash"}
	runnerConfig.ContainerConfig.Cmd = []string{"-c", command}
	log.Println("issuing container command : '" + command + "'")

	// List networks and fish out the integration network ID
	filters := filters.NewArgs()
	filters.Add("name", "integration")

	networks, err := client.NetworkList(context.Background(), types.NetworkListOptions{Filters: filters})
	if err != nil {
		return retry.UnexpectedError(err)
	}

	if len(networks) == 0 {
		return retry.UnexpectedError(errors.New("integration network not found"))
	}

	// Create container, attach it to the integration network, and wait for completion
	resp, err := client.ContainerCreate(ctx, runnerConfig.ContainerConfig, runnerConfig.HostConfig, nil, "")
	if err != nil {
		return retry.UnexpectedError(err)
	}

	if err = client.NetworkConnect(ctx, networks[0].ID, resp.ID, nil); err != nil {
		return retry.UnexpectedError(err)
	}

	if err = client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return retry.UnexpectedError(err)
	}

	_, err = client.ContainerWait(ctx, resp.ID)
	if err != nil {
		return retry.UnexpectedError(err)
	}

	// Gather container logs and output them
	out, err := client.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true})
	if err != nil {
		return retry.UnexpectedError(err)
	}

	_, err = io.Copy(os.Stdout, out)
	if err != nil {
		return retry.UnexpectedError(err)
	}

	// Check return code of container and return err if it was non-zero
	inspect, err := client.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return retry.UnexpectedError(err)
	}

	if inspect.State.ExitCode != 0 {
		return retry.ExpectedError(errors.New("container exited with non-zero code"))
	}

	return nil
}

// CommandInContainerWithTimeout is a wrapper around commandInContainer that retries until we hit the timeout value
func CommandInContainerWithTimeout(ctx context.Context, client *client.Client, runnerConfig *ContainerConfigs, command string, timeout int) error {
	err := retry.Constant(time.Duration(timeout)*time.Second, retry.WithUnits(10*time.Second)).Retry(func() error {
		if commandErr := CommandInContainer(ctx, client, runnerConfig, command); commandErr != nil {
			return commandErr
		}

		return nil
	})

	return err
}
