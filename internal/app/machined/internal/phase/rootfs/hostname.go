/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package rootfs

import (
	"log"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/machined/internal/phase/rootfs/etc"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
)

// Hostname represents the Hostname task.
type Hostname struct{}

// NewHostnameTask initializes and returns an Hostname task.
func NewHostnameTask() phase.Task {
	return &Hostname{}
}

// RuntimeFunc returns the runtime function.
func (task *Hostname) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

func (task *Hostname) runtime(args *phase.RuntimeArgs) (err error) {
	// Create /etc/hosts and set hostname.
	// Priority is:
	// 1. Config (explicitly defined by the user)
	// 2. Kernel arg
	// 3. Platform
	// 4. DHCP
	// 5. Default with the format: talos-<ip addr>
	kernelHostname := kernel.ProcCmdline().Get(constants.KernelParamHostname).First()

	var platformHostname []byte

	platformHostname, err = args.Platform().Hostname()
	if err != nil {
		return err
	}

	configHostname := args.Config().Machine().Network().Hostname()

	switch {
	case configHostname != "":
		log.Printf("using hostname from config: %s\n", configHostname)
	case kernelHostname != nil:
		args.Config().Machine().Network().SetHostname(*kernelHostname)
		log.Printf("using hostname provide via kernel arg: %s\n", *kernelHostname)
	case platformHostname != nil:
		args.Config().Machine().Network().SetHostname(string(platformHostname))
		log.Printf("using hostname provided via platform: %s\n", string(platformHostname))

		// case data.Networking.OS.Hostname != "":
		// 	d.Networking.OS.Hostname = data.Networking.OS.Hostname
		// 	log.Printf("dhcp hostname %s:", data.Networking.OS.Hostname)
	} //nolint: wsl

	return etc.Hosts(args.Config().Machine().Network().Hostname())
}
