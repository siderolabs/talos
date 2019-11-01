// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pkg

import (
	"log"
	"path/filepath"

	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"
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
}

// Install installs Talos.
func Install(p runtime.Platform, config runtime.Configurator, sequence runtime.Sequence, opts *InstallOptions) (err error) {
	cmdline := kernel.NewCmdline("")
	cmdline.Append("initrd", filepath.Join("/", "default", constants.InitramfsAsset))
	cmdline.Append(constants.KernelParamPlatform, p.Name())
	cmdline.Append(constants.KernelParamConfig, opts.ConfigSource)

	if err = cmdline.AppendAll(p.KernelArgs().Strings()); err != nil {
		return err
	}

	if err = cmdline.AppendAll(config.Machine().Install().ExtraKernelArgs()); err != nil {
		return err
	}

	cmdline.AppendDefaults()

	i, err := NewInstaller(cmdline, config.Machine().Install())
	if err != nil {
		return err
	}

	if err = i.Install(sequence); err != nil {
		return err
	}

	log.Printf("installation of %s complete", version.Tag)

	return nil
}
