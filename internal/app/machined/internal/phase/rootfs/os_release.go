/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs/etc"
	"github.com/talos-systems/talos/internal/app/machined/internal/platform"
	"github.com/talos-systems/talos/internal/app/machined/internal/runtime"
	"github.com/talos-systems/talos/pkg/userdata"
)

// OSRelease represents the OSRelease task.
type OSRelease struct{}

// NewOSReleaseTask initializes and returns an OSRelease task.
func NewOSReleaseTask() phase.Task {
	return &OSRelease{}
}

// RuntimeFunc returns the runtime function.
func (task *OSRelease) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *OSRelease) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	// Create /etc/os-release.
	return etc.OSRelease()
}
