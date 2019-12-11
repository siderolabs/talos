// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// To disable the `stutter` check
// nolint: golint
package container

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/test-framework/pkg/checker"
	"github.com/talos-systems/talos/internal/test-framework/pkg/runner"
	"github.com/talos-systems/talos/pkg/retry"
)

// ContainerRunner implements the Runner interface.
// ContainerRunner contains the container configuration and client.
type ContainerRunner struct {
	ClusterName string
	Container   *container.Config
	Host        *container.HostConfig
	Client      *client.Client
}

// New creates a new ContainerRunner with the specified options and
// pulls the necessary image.
func New(setters ...Option) (runner.Runner, error) {
	var (
		err    error
		reader io.ReadCloser
		result *multierror.Error
	)

	cr := defaultOptions()

	for _, setter := range setters {
		result = multierror.Append(result, setter(cr))
	}

	if reader, err = cr.Client.ImagePull(context.Background(), cr.Container.Image, types.ImagePullOptions{}); err != nil {
		result = multierror.Append(result, err)
	}

	// nolint: errcheck
	defer reader.Close()

	if _, err = io.Copy(ioutil.Discard, reader); err != nil {
		result = multierror.Append(result, err)
	}

	return cr, result.ErrorOrNil()
}

// Run invokes a one off command inside the container.
func (cr *ContainerRunner) Run(ctx context.Context, check checker.Check) (err error) {
	var integrationContainer string

	if integrationContainer, err = cr.prepareExec(ctx); err != nil {
		return err
	}

	log.Println(check.Name)

	return cr.runCommandFn(ctx, &check, integrationContainer)()
}

// Check invokes a command with a specified check function to validate
// the command was executed correctly.
func (cr *ContainerRunner) Check(ctx context.Context, check checker.Check) (err error) {
	var integrationContainer string

	if integrationContainer, err = cr.prepareExec(ctx); err != nil {
		return err
	}

	log.Println(check.Name)
	err = retry.Exponential(check.Wait, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(cr.runCommandFn(ctx, &check, integrationContainer))

	// Add in a quick print of the output when things are successful
	if err == nil {
		log.Printf("Stdout:\n%s", check.Stdout.String())
	}

	return err
}

// Cleanup is used to remove the container and any associated resources for the runner.
func (cr *ContainerRunner) Cleanup(ctx context.Context) error {
	integrationContainer, err := cr.Name(ctx)
	if err != nil {
		return err
	}

	if err = cr.Client.ContainerStop(ctx, integrationContainer, nil); err != nil {
		return err
	}

	return cr.Client.ContainerRemove(ctx, integrationContainer, types.ContainerRemoveOptions{})
}

// Start creates a container we can use for all of our checks.
func (cr *ContainerRunner) start(ctx context.Context) error {
	// Test for Network
	networkFilters := filters.NewArgs()
	networkFilters.Add("name", cr.ClusterName)

	networks, err := cr.Client.NetworkList(ctx, types.NetworkListOptions{Filters: networkFilters})
	if err != nil {
		return err
	}

	if len(networks) == 0 {
		return fmt.Errorf("%s network not found", cr.ClusterName)
	}

	var exists bool
	if exists, err = cr.exists(ctx); err != nil {
		return err
	}

	if exists {
		return nil
	}

	// Define base entrypoint and command for checks
	// We use `/bin/bash -c "sleep 10000"` as our entrypoint so we can keep
	// the container running for the length of the integration test run.
	cr.Container.Entrypoint = []string{"/bin/bash"}
	cr.Container.Cmd = []string{"-c", "sleep 10000"}

	// Create container, attach it to the integration network, and wait for completion
	resp, err := cr.Client.ContainerCreate(ctx, cr.Container, cr.Host, nil, "")
	if err != nil {
		return err
	}

	if err = cr.Client.NetworkConnect(ctx, networks[0].ID, resp.ID, nil); err != nil {
		return err
	}

	if err = cr.Client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		return err
	}

	return nil
}

// Name discovers the runner container and returns the name of the container.
func (cr *ContainerRunner) Name(ctx context.Context) (string, error) {
	containers, err := cr.findRunnerContainer(ctx)
	if err != nil {
		return "", err
	}

	if len(containers) > 1 {
		return "", fmt.Errorf("multiple containers found matching %s=%s", "talos.integration.name", cr.ClusterName)
	}

	if len(containers) == 0 {
		return "", nil
	}

	// No idea why Names is []string
	return containers[0].Names[0], nil
}

// exists checks to see if there is only a single runner container available.
// If more than one container is returned from our search, we error. Otherwise
// we'll use the container if it exists or create a new one if it does not.
func (cr *ContainerRunner) exists(ctx context.Context) (bool, error) {
	containers, err := cr.findRunnerContainer(ctx)
	if err != nil {
		return false, err
	}

	if len(containers) > 1 {
		return false, err
	}

	if len(containers) == 0 {
		return false, nil
	}

	return true, nil
}

// findRunnerContainer discovers the container to be used for command execution
// by searching through labels `talos.owned` and `talos.integration.name`.
func (cr *ContainerRunner) findRunnerContainer(ctx context.Context) ([]types.Container, error) {
	// Test if integration container already exists
	containerFilters := filters.NewArgs()
	containerFilters.Add("label", fmt.Sprintf("%s=%s", "talos.owned", "true"))
	containerFilters.Add("label", fmt.Sprintf("%s=%s", "talos.integration.name", cr.ClusterName))

	return cr.Client.ContainerList(ctx, types.ContainerListOptions{Filters: containerFilters})
}

// prepareExec ensures that the runner container is running.
func (cr *ContainerRunner) prepareExec(ctx context.Context) (string, error) {
	err := cr.start(ctx)
	if err != nil {
		return "", err
	}

	return cr.Name(ctx)
}

// runCommandFn sets up the actual command to be run inside the container.
func (cr *ContainerRunner) runCommandFn(ctx context.Context, check *checker.Check, container string) func() error {
	return func() error {
		var (
			err     error
			idresp  types.IDResponse
			inspect types.ContainerExecInspect
			resp    types.HijackedResponse
		)

		execConfig := types.ExecConfig{
			Tty:          true,
			AttachStdin:  true,
			AttachStdout: true,
			AttachStderr: true,
			Cmd:          []string{"/bin/bash", "-c", check.Command.String()},
		}

		idresp, err = cr.Client.ContainerExecCreate(ctx, container, execConfig)
		if err != nil {
			return retry.UnexpectedError(err)
		}

		resp, err = cr.Client.ContainerExecAttach(ctx, idresp.ID, types.ExecStartCheck{})
		if err != nil {
			return retry.UnexpectedError(err)
		}

		// Reset stdout/stderr
		check.Stdout.Reset()
		check.Stderr.Reset()

		_, err = stdcopy.StdCopy(&check.Stdout, &check.Stderr, resp.Reader)
		if err != nil {
			return retry.UnexpectedError(err)
		}

		// Wait for command to complete
		err = retry.Constant(check.Wait, retry.WithUnits(250*time.Millisecond), retry.WithJitter(50*time.Millisecond)).Retry(func() error {
			// Make sure command ran correctly
			inspect, err = cr.Client.ContainerExecInspect(ctx, idresp.ID)
			if err != nil {
				return retry.UnexpectedError(err)
			}
			if inspect.Running {
				return retry.ExpectedError(fmt.Errorf("check %q still running", check.Command.String()))
			}
			return nil
		})
		if err != nil {
			return retry.ExpectedError(fmt.Errorf("check %q still running", check.Command.String()))
		}

		if inspect.ExitCode != 0 {
			return retry.ExpectedError(fmt.Errorf("check %q failed with exit status %d\nstderr:%s\nstdout:%s", check.Command.String(), inspect.ExitCode, check.Stderr.String(), check.Stdout.String()))
		}

		// Check our check
		if !check.Check(check.Stdout.String()) {
			return retry.ExpectedError(fmt.Errorf("check was not successful\nstderr:%s\nstdout:%s", check.Stderr.String(), check.Stdout.String()))
		}

		return nil
	}
}
