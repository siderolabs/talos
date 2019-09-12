/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package upgrade

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/install"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/internal/app/machined/proto"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Upgrade represents the task for stop all containerd tasks in the
// k8s.io namespace.
type Upgrade struct {
	devname string
	ref     string
}

// NewUpgradeTask initializes and returns an Services task.
func NewUpgradeTask(devname string, req *proto.UpgradeRequest) phase.Task {
	return &Upgrade{
		devname: devname,
		ref:     req.Image,
	}
}

// RuntimeFunc returns the runtime function.
func (task *Upgrade) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return func(platform platform.Platform, data *userdata.UserData) error {
		return task.standard(platform)
	}
}

func (task *Upgrade) standard(platform platform.Platform) (err error) {
	// TODO(andrewrynhard): To handle cases when the newer version changes the
	// platform name, this should be determined in the installer container.
	var userdata *string
	if userdata = kernel.ProcCmdline().Get(constants.KernelParamUserData).First(); userdata == nil {
		return errors.Errorf("no user data option was found")
	}
	if err = install.Install(task.ref, task.devname, strings.ToLower(platform.Name())); err != nil {
		return err
	}

	return nil
}
