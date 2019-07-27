/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package install

import (
	"log"

	"github.com/talos-systems/talos/internal/app/machined/internal/mount"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Install represents the Install task.
type Install struct{}

// NewInstallTask initializes and returns an Install task.
func NewInstallTask() phase.Task {
	return &Install{}
}

// RuntimeFunc returns the runtime function.
func (task *Install) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Standard:
		return task.runtime
	default:
		return nil
	}
}

func (task *Install) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	// Perform any tasks required by a particular platform.
	log.Printf("performing platform specific tasks")
	if err = platform.Prepare(data); err != nil {
		return err
	}

	var initializer *mount.Initializer
	if initializer, err = mount.NewInitializer(""); err != nil {
		return err
	}

	// Mount the owned partitions.
	log.Printf("mounting the owned partitions")
	if err = initializer.InitOwned(); err != nil {
		return err
	}

	// Install handles additional system setup
	if err = platform.Install(data); err != nil {
		return err
	}

	return nil
}
