// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"os"

	"github.com/talos-systems/talos/cmd/installer/pkg/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/kubernetes"
)

// Uncordon represents the task for stop all containerd tasks in the
// k8s.io namespace.
type Uncordon struct{}

// NewUncordonTask initializes and returns an Services task.
func NewUncordonTask() phase.Task {
	return &Uncordon{}
}

// TaskFunc returns the runtime function.
func (task *Uncordon) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *Uncordon) standard(r runtime.Runtime) (err error) {
	f, err := os.Open(syslinux.SyslinuxLdlinux)
	if err != nil {
		return err
	}

	// nolint: errcheck
	defer f.Close()

	adv, err := syslinux.NewADV(f)
	if err != nil {
		return err
	}

	_, ok := adv.ReadTag(syslinux.AdvUpgrade)
	if !ok {
		return nil
	}

	var hostname string

	if hostname, err = os.Hostname(); err != nil {
		return err
	}

	var kubeHelper *kubernetes.Client

	if kubeHelper, err = kubernetes.NewClientFromKubeletKubeconfig(); err != nil {
		return err
	}

	if err = kubeHelper.Uncordon(hostname); err != nil {
		return err
	}

	return nil
}
