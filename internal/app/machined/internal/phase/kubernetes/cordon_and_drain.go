/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kubernetes

import (
	"os"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/userdata"
)

// CordonAndDrain represents the task for stop all containerd tasks in the
// k8s.io namespace.
type CordonAndDrain struct{}

// NewCordonAndDrainTask initializes and returns an Services task.
func NewCordonAndDrainTask() phase.Task {
	return &CordonAndDrain{}
}

// RuntimeFunc returns the runtime function.
func (task *CordonAndDrain) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return func(platform platform.Platform, data *userdata.UserData) error {
		return task.standard()
	}
}

func (task *CordonAndDrain) standard() (err error) {
	var hostname string
	if hostname, err = os.Hostname(); err != nil {
		return err
	}

	var kubeHelper *kubernetes.Helper
	if kubeHelper, err = kubernetes.NewHelper(); err != nil {
		return err
	}

	if err = kubeHelper.CordonAndDrain(hostname); err != nil {
		return err
	}

	return nil
}
