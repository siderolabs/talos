// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"time"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

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

	// We remove pods with POD network mode first so that the CNI can perform
	// any cleanup tasks. If we don't do this, we run the risk of killing the
	// CNI, preventing the CRI from cleaning up the pod's netwokring.

	if err = client.RemovePodSandboxes(runtimeapi.NamespaceMode_POD, runtimeapi.NamespaceMode_CONTAINER); err != nil {
		return err
	}

	// With the POD network mode pods out of the way, we kill the remaining
	// pods.

	if err = client.RemovePodSandboxes(); err != nil {
		return err
	}

	return nil
}
