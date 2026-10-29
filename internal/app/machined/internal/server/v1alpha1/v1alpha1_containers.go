// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"cmp"
	"context"
	"fmt"
	"syscall"
	"time"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/errdefs"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/internal/ctrhelper"
	containerdrunner "github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/containerd"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	containersres "github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// restartContainer stops a container declared with a ContainerConfig as the controllers stop it:
// SIGTERM, and SIGKILL once its stop timeout is over. It is restarted once it has exited, as after
// any exit.
func (s *Server) restartContainer(ctx context.Context, id string) error {
	stopTimeout, err := s.containerStopTimeout(ctx, id)
	if err != nil {
		return err
	}

	ctx, detachedCtx, c8dClient, err := ctrhelper.ContainerdInstanceHelper(ctx, &common.ContainerdInstance{
		Driver:    common.ContainerDriver_CONTAINERD,
		Namespace: common.ContainerdNamespace_NS_TALOSCONTAINERS,
	})
	if err != nil {
		return err
	}

	task, container, err := loadTask(ctx, c8dClient, id)
	if err != nil {
		c8dClient.Close() //nolint:errcheck

		return err
	}

	waitCtx, cancel := context.WithTimeout(detachedCtx, stopTimeout+containerdrunner.KillWaitTimeout)

	statusC, err := task.Wait(waitCtx)
	if err != nil {
		cancel()
		c8dClient.Close() //nolint:errcheck

		return fmt.Errorf("failed to wait for container %q: %w", id, err)
	}

	if err = task.Kill(ctx, syscall.SIGTERM, containerdapi.WithKillAll); err != nil {
		cancel()
		c8dClient.Close() //nolint:errcheck

		return fmt.Errorf("failed to stop container %q: %w", id, err)
	}

	logger := s.Logger.With(zap.String("instance", id), zap.Uint32("pid", task.Pid()))

	go func() {
		defer c8dClient.Close() //nolint:errcheck
		defer cancel()

		killAfterStopTimeout(waitCtx, logger, container, task.Pid(), statusC, stopTimeout)
	}()

	return nil
}

// containerStopTimeout returns the stop timeout of a container instance.
func (s *Server) containerStopTimeout(ctx context.Context, id string) (time.Duration, error) {
	instanceSpec, err := safe.StateGetByID[*containersres.ContainerInstanceSpec](ctx, s.Controller.Runtime().State().V1Alpha2().Resources(), id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return constants.ContainerDefaultStopTimeout, nil
		}

		return 0, err
	}

	return cmp.Or(instanceSpec.TypedSpec().StopTimeout, constants.ContainerDefaultStopTimeout), nil
}

// killAfterStopTimeout kills the task of the container with SIGKILL if it hasn't exited by the end of its stop timeout.
func killAfterStopTimeout(
	ctx context.Context, logger *zap.Logger, container containerdapi.Container, pid uint32, statusC <-chan containerdapi.ExitStatus, stopTimeout time.Duration,
) {
	if exitedWithin(ctx, statusC, stopTimeout) {
		return
	}

	// the instance ID is reused when a container is declared again, so it has to be the same task
	task, err := container.Task(ctx, nil)
	if err != nil || task.Pid() != pid {
		return
	}

	logger.Info("killing container after its stop timeout", zap.Duration("stop_timeout", stopTimeout))

	if err = task.Kill(ctx, syscall.SIGKILL, containerdapi.WithKillAll); err != nil && !errdefs.IsNotFound(err) {
		logger.Warn("failed to kill container", zap.Error(err))
	}
}

func loadTask(ctx context.Context, c8dClient *containerdapi.Client, id string) (containerdapi.Task, containerdapi.Container, error) {
	container, err := c8dClient.LoadContainer(ctx, id)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, nil, fmt.Errorf("container %q not found", id)
		}

		return nil, nil, err
	}

	task, err := container.Task(ctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			return nil, nil, fmt.Errorf("container %q not found", id)
		}

		return nil, nil, err
	}

	return task, container, nil
}

// exitedWithin reports whether the task exited within timeout, or ctx was canceled.
//
// A failed wait (e.g. containerd restarting) is no exit, the timeout decides then.
func exitedWithin(ctx context.Context, statusC <-chan containerdapi.ExitStatus, timeout time.Duration) bool {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		select {
		case status, ok := <-statusC:
			if ok && status.Error() == nil {
				return true
			}

			statusC = nil
		case <-timer.C:
			select {
			case status, ok := <-statusC:
				return ok && status.Error() == nil
			default:
				return false
			}
		case <-ctx.Done():
			return true
		}
	}
}
