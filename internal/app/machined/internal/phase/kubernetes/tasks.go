/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kubernetes

import (
	"context"
	"log"
	"syscall"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/namespaces"
	"github.com/pkg/errors"
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

// RuntimeFunc returns the runtime function.
func (task *KillKubernetesTasks) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return func(args *phase.RuntimeArgs) error {
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

	var g errgroup.Group

	for _, task := range response.Tasks {
		task := task // https://golang.org/doc/faq#closures_and_goroutines
		log.Printf("killing task %s", task.ID)
		g.Go(func() error {
			if _, err = s.Kill(ctx, &tasks.KillRequest{ContainerID: task.ID, Signal: uint32(syscall.SIGTERM), All: true}); err != nil {
				return errors.Wrap(err, "error killing task")
			}
			// TODO(andrewrynhard): Send SIGKILL on a timeout threshold.
			if _, err = s.Wait(ctx, &tasks.WaitRequest{ContainerID: task.ID}); err != nil {
				return errors.Wrap(err, "error waiting on task")
			}
			if _, err = s.Delete(ctx, &tasks.DeleteTaskRequest{ContainerID: task.ID}); err != nil {
				return errors.Wrap(err, "error deleting task")
			}

			return nil
		})
	}

	return g.Wait()
}
