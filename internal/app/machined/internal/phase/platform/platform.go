/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package platform

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/platform"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/runtime/initializer"
	"github.com/talos-systems/talos/pkg/userdata"
)

// Platform represents the Platform task.
type Platform struct{}

// NewPlatformTask initializes and returns an Platform task.
func NewPlatformTask() phase.Task {
	return &Platform{}
}

// RuntimeFunc returns the runtime function.
func (task *Platform) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	return task.runtime
}

func (task *Platform) runtime(platform platform.Platform, data *userdata.UserData) (err error) {
	i, err := initializer.New(platform.Mode())
	if err != nil {
		return err
	}
	if err = i.Initialize(platform, data); err != nil {
		return err
	}

	// Attempt to retrieve hostname via platform metadata
	if data.Networking == nil {
		data.Networking = &userdata.Networking{}
	}
	if data.Networking.OS == nil {
		data.Networking.OS = &userdata.OSNet{}
	}

	hostname, err := platform.Hostname()
	if err != nil {
		return err
	}

	if hostname != nil {
		data.Networking.OS.Hostname = string(hostname)
	}

	// Attempt to identify external addresses assigned to the instance via platform
	// metadata
	addrs, err := platform.ExternalIPs()
	if err != nil {
		return err
	}

	// And add them to our cert SANs for trustd
	for _, addr := range addrs {
		data.Services.Trustd.CertSANs = append(data.Services.Trustd.CertSANs, addr.String())
	}

	return nil
}
