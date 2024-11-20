// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package vmware provides the VMware platform implementation.
package vmware

import (
	"github.com/siderolabs/go-procfs/procfs"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// VMware is the concrete type that implements the platform.Platform interface.
type VMware struct{}

// Name implements the platform.Platform interface.
func (v *VMware) Name() string {
	return "vmware"
}

// Mode implements the platform.Platform interface.
func (v *VMware) Mode() runtime.Mode {
	return runtime.ModeCloud
}

// KernelArgs implements the runtime.Platform interface.
func (v *VMware) KernelArgs(arch string) procfs.Parameters {
	switch arch {
	case "amd64":
		return []*procfs.Parameter{
			procfs.NewParameter(constants.KernelParamConfig).Append(constants.ConfigGuestInfo),
			procfs.NewParameter("console").Append("tty0").Append("ttyS0"),
			procfs.NewParameter("earlyprintk").Append("ttyS0,115200"),
			procfs.NewParameter(constants.KernelParamNetIfnames).Append("0"),
		}
	default:
		return nil // not supported on !amd64
	}
}
