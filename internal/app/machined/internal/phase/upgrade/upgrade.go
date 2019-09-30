/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package upgrade

import (
	"strings"

	"github.com/pkg/errors"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Upgrade represents the task for stop all containerd tasks in the
// k8s.io namespace.
type Upgrade struct {
	devname string
	ref     string
}

// NewUpgradeTask initializes and returns an Services task.
func NewUpgradeTask(devname string, req *machineapi.UpgradeRequest) phase.Task {
	return &Upgrade{
		devname: devname,
		ref:     req.Image,
	}
}

// RuntimeFunc returns the runtime function.
func (task *Upgrade) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.standard
}

func (task *Upgrade) standard(args *phase.RuntimeArgs) (err error) {
	// TODO(andrewrynhard): To handle cases when the newer version changes the
	// platform name, this should be determined in the installer container.
	var config *string
	if config = kernel.ProcCmdline().Get(constants.KernelParamConfig).First(); config == nil {
		return errors.Errorf("no config option was found")
	}
	if err = install.Install(task.ref, task.devname, strings.ToLower(args.Platform().Name())); err != nil {
		return err
	}

	return nil
}
