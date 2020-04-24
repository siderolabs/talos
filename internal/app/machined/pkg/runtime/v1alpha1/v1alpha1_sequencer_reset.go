// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/phase"
)

// Reset is the reset sequence.
func (*Sequencer) Reset(in *machine.ResetRequest) []runtime.Phase {
	if in.Graceful {
		return []runtime.Phase{
			&phase.CordonAndDrainNode{},
			&phase.LeaveEtcd{Preserve: false},
			&phase.RemoveAllPods{},
			&phase.StopServices{},
			&phase.TeardownFilesystems{},
			&phase.UnmountSystemDisks{},
			&phase.UnmountSystemDiskBindMounts{},
			&phase.ResetSystemDisk{},
		}
	}

	return []runtime.Phase{
		&phase.StopServices{},
		&phase.TeardownFilesystems{},
		&phase.UnmountSystemDisks{},
		&phase.UnmountSystemDiskBindMounts{},
		&phase.ResetSystemDisk{},
	}
}
