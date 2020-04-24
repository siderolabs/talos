// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"log"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/universe"
	"github.com/talos-systems/talos/pkg/version"
)

// InstallOptions represents the set of options available for an install.
type InstallOptions struct {
	ConfigSource    string
	Disk            string
	Platform        string
	ExtraKernelArgs []string
	Bootloader      bool
	Upgrade         bool
	Force           bool
}

// Install installs Talos.
func Install(p runtime.Platform, config runtime.Configurator, sequence runtime.Sequence, opts *InstallOptions) (err error) {
	cmdline := procfs.NewCmdline("")
	cmdline.Append(universe.KernelParamPlatform, p.Name())
	cmdline.Append(universe.KernelParamConfig, opts.ConfigSource)

	if err = cmdline.AppendAll(p.KernelArgs().Strings()); err != nil {
		return err
	}

	if err = cmdline.AppendAll(config.Machine().Install().ExtraKernelArgs()); err != nil {
		return err
	}

	cmdline.AppendDefaults()

	i, err := NewInstaller(cmdline, sequence, config.Machine().Install())
	if err != nil {
		return err
	}

	if err = i.Install(sequence); err != nil {
		return err
	}

	log.Printf("installation of %s complete", version.Tag)

	return nil
}
