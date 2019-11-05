// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package upgrade

import (
	"log"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/config/types/v1alpha1"
)

// Upgrade represents the task for stop all containerd tasks in the
// k8s.io namespace.
type Upgrade struct {
	disk  string
	image string
}

// NewUpgradeTask initializes and returns an Services task.
func NewUpgradeTask(devname string, req *machineapi.UpgradeRequest) phase.Task {
	return &Upgrade{
		disk:  devname,
		image: req.Image,
	}
}

// TaskFunc returns the runtime function.
func (task *Upgrade) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.standard
}

func (task *Upgrade) standard(r runtime.Runtime) (err error) {
	log.Printf("performing upgrade via %q", task.image)

	c := r.Config()
	if cfg, ok := c.(*v1alpha1.Config); ok {
		cfg.MachineConfig.MachineInstall.InstallDisk = task.disk
		cfg.MachineConfig.MachineInstall.InstallImage = task.image

		r = runtime.NewRuntime(r.Platform(), runtime.Configurator(cfg), runtime.Upgrade)
	}

	if err = install.Install(r); err != nil {
		return err
	}

	return nil
}
