// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"
	"log"
	"syscall"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types/task"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"golang.org/x/sync/errgroup"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// KillKubernetesTasks represents the task for stop all containerd tasks in the
// k8s.io namespace.
type KillKubernetesTasks struct{}

// NewKillKubernetesTasksTask initializes and returns an Services task.
func NewKillKubernetesTasksTask() phase.Task {
	return &KillKubernetesTasks{}
}

// TaskFunc returns the runtime function.
func (task *KillKubernetesTasks) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return func(r runtime.Runtime) error {
		return task.standard()
	}
}

func (task *KillKubernetesTasks) standard() (err error) {
	if err = system.Services(nil).Stop(context.Background(), "kubelet"); err != nil {
		return err
	}

	client, err := containerd.New(constants.ContainerdAddress)
	if err != nil {
		return err
	}

	s := client.TaskService()

	ctx := namespaces.WithNamespace(context.Background(), "k8s.io")

	response, err := s.List(ctx, &tasks.ListTasksRequest{})
	if err != nil {
		return err
	}

	sigtermCtx, sigtermCancel := context.WithCancel(ctx)

	defer sigtermCancel()

	var g errgroup.Group

	for _, task := range response.Tasks {
		task := task // https://golang.org/doc/faq#closures_and_goroutines

		g.Go(func() error {
			done := make(chan bool, 1)

			defer close(done)

			// Make a best effort attempt at killing the task gracefully.
			go func() {
				if err := stop(sigtermCtx, s, task, syscall.SIGTERM); err != nil {
					return
				}

				done <- true
			}()

			select {
			case <-done:
			case <-time.After(time.Minute):
				// Cancel the SIGTERM attempt.
				sigtermCancel()

				// Delete the task forcefully after a timeout.
				if err := stop(ctx, s, task, syscall.SIGKILL); err != nil {
					if !errdefs.IsNotFound(err) {
						return fmt.Errorf("error stopping task %s: %w", task.ID, err)
					}
				}
			}

			return nil
		})
	}

	return g.Wait()
}

func stop(ctx context.Context, s tasks.TasksClient, task *task.Process, sig syscall.Signal) (err error) {
	if _, err = s.Kill(ctx, &tasks.KillRequest{ContainerID: task.ID, Signal: uint32(sig), All: true}); err != nil {
		return err
	}

	var r *tasks.WaitResponse

	if r, err = s.Wait(ctx, &tasks.WaitRequest{ContainerID: task.ID}); err != nil {
		return err
	}

	if _, err = s.Delete(ctx, &tasks.DeleteTaskRequest{ContainerID: task.ID}); err != nil {
		return err
	}

	log.Printf("task %s %s with exit code: %d", task.ID, sig.String(), r.ExitStatus)

	return nil
}
