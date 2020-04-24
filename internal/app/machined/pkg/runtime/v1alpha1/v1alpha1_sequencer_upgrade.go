// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/phase"
)

// Upgrade is the upgrade sequence.
func (*Sequencer) Upgrade(in *machine.UpgradeRequest) []runtime.Phase {
	return []runtime.Phase{
		&phase.CordonAndDrainNode{},
		&phase.LeaveEtcd{Preserve: in.GetPreserve()},
		&phase.RemoveAllPods{},
		&phase.StopServices{},
		&phase.TeardownFilesystems{},
		&phase.UnmountSystemDisks{},
		&phase.UnmountSystemDiskBindMounts{},
		&phase.VerifySystemDiskAvailability{},
		&phase.Upgrade{},
		&phase.StopServices{},
	}
}
