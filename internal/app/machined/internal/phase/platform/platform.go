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

	hostname, err := args.Platform().Hostname()
	if err != nil {
		return err
	}

	if hostname != nil {
		args.Config().Machine().Network().SetHostname(string(hostname))
	}

	addrs, err := args.Platform().ExternalIPs()
	if err != nil {
		return err
	}

	sans := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		sans = append(sans, addr.String())
	}

	args.Config().Machine().Security().SetCertSANs(sans)
	args.Config().Cluster().SetCertSANs(sans)

	return nil
}
