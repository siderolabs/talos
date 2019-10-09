/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package platform

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/runtime/initializer"
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

func (task *Platform) runtime(args *phase.RuntimeArgs) (err error) {
	i, err := initializer.New(args.Platform().Mode())
	if err != nil {
		return err
	}

	if err = i.Initialize(args.Platform(), args.Config().Machine().Install()); err != nil {
		return err
	}

	_, err = args.Platform().Hostname()
	if err != nil {
		return err
	}

	// if hostname != nil {
	// 	data.Networking.OS.Hostname = string(hostname)
	// }

	// // Attempt to identify external addresses assigned to the instance via platform
	// // metadata
	// addrs, err := platform.ExternalIPs()
	// if err != nil {
	// 	return err
	// }

	// // And add them to our cert SANs for trustd
	// for _, addr := range addrs {
	// 	data.Services.Trustd.CertSANs = append(data.Services.Trustd.CertSANs, addr.String())
	// }

	return nil
}
