// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//go:generate go tool github.com/dmarkham/enumer -type=MachineStage -linecomment -text

// MachineStage describes the stage of the machine boot/run process.
type MachineStage int

// Machine stages.
//
//structprotogen:gen_enum
const (
	MachineStageUnknown      MachineStage = iota // unknown
	MachineStageBooting                          // booting
	MachineStageInstalling                       // installing
	MachineStageMaintenance                      // maintenance
	MachineStageRunning                          // running
	MachineStageRebooting                        // rebooting
	MachineStageShuttingDown                     // shutting down
	MachineStageResetting                        // resetting
	MachineStageUpgrading                        // upgrading
)
