// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"log"
	"time"

	"golang.org/x/sync/errgroup"
	"k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/pkg/system"
	"github.com/talos-systems/talos/internal/pkg/cri"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// RemoveAllPods represents the task for stopping all pods.
type RemoveAllPods struct{}

// NewRemoveAllPodsTask initializes and returns an Services task.
func NewRemoveAllPodsTask() phase.Task {
	return &RemoveAllPods{}
}

// TaskFunc returns the runtime function.
func (task *RemoveAllPods) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return func(r runtime.Runtime) error {
		return task.standard()
	}
}

func (task *RemoveAllPods) standard() (err error) {
	if err = system.Services(nil).Stop(context.Background(), "kubelet"); err != nil {
		return err
	}

	client, err := cri.NewClient("unix://"+constants.ContainerdAddress, 10*time.Second)
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer client.Close()

	ctx := context.Background()

	pods, err := client.ListPodSandbox(ctx, nil)
	if err != nil {
		return err
	}

	var g errgroup.Group

	for _, pod := range pods {
		pod := pod // https://golang.org/doc/faq#closures_and_goroutines

		g.Go(func() error {
			if err := remove(ctx, client, pod); err != nil {
				return err
			}

			return nil
		})
	}

	return g.Wait()
}

func remove(ctx context.Context, client *cri.Client, pod *v1alpha2.PodSandbox) (err error) {
	log.Printf("removing pod %s/%s", pod.Metadata.Namespace, pod.Metadata.Name)

	filter := &v1alpha2.ContainerFilter{
		PodSandboxId: pod.Id,
	}

	containers, err := client.ListContainers(ctx, filter)
	if err != nil {
		return err
	}

	var g errgroup.Group

	for _, container := range containers {
		container := container // https://golang.org/doc/faq#closures_and_goroutines

		g.Go(func() error {
			log.Printf("removing container %s/%s:%s", pod.Metadata.Namespace, pod.Metadata.Name, container.Metadata.Name)

			// TODO(andrewrynhard): Can we set the timeout dynamically?
			if err = client.StopContainer(ctx, container.Id, 30); err != nil {
				return err
			}

			if err = client.RemoveContainer(ctx, container.Id); err != nil {
				return err
			}

			log.Printf("removed container %s/%s:%s", pod.Metadata.Namespace, pod.Metadata.Name, container.Metadata.Name)

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return err
	}

	if err = client.StopPodSandbox(ctx, pod.Id); err != nil {
		return err
	}

	if err = client.RemovePodSandbox(ctx, pod.Id); err != nil {
		return err
	}

	log.Printf("removed pod %s/%s", pod.Metadata.Namespace, pod.Metadata.Name)

	return nil
}
