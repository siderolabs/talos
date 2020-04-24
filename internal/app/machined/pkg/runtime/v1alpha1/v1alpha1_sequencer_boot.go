// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/phase"
)

// Boot is the boot sequence.
func (*Sequencer) Boot() []runtime.Phase {
	return []runtime.Phase{
		&phase.ValidateConfig{},
		&phase.SetUserEnvVars{},
		&phase.StartStage1SystemServices{},
		&phase.InitializePlatform{},
		&phase.StartStage2SystemServices{},
		&phase.VerifyInstallation{},
		&phase.SetupFilesystems{},
		&phase.MountUserDisks{},
		&phase.UserRequests{},
		&phase.StartOrchestrationServices{},
		&phase.LabelNode{},
		&phase.UpdateBootloader{},
	}
}
