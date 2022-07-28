// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/machinery/api/machine"
	"github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

func TestMachineStageMatchesProto(t *testing.T) {
	assert := assert.New(t)

	assert.EqualValues(runtime.MachineStageUnknown, machine.MachineStatusEvent_UNKNOWN)
	assert.EqualValues(runtime.MachineStageBooting, machine.MachineStatusEvent_BOOTING)
	assert.EqualValues(runtime.MachineStageInstalling, machine.MachineStatusEvent_INSTALLING)
	assert.EqualValues(runtime.MachineStageMaintenance, machine.MachineStatusEvent_MAINTENANCE)
	assert.EqualValues(runtime.MachineStageRunning, machine.MachineStatusEvent_RUNNING)
	assert.EqualValues(runtime.MachineStageRebooting, machine.MachineStatusEvent_REBOOTING)
	assert.EqualValues(runtime.MachineStageShuttingDown, machine.MachineStatusEvent_SHUTTING_DOWN)
	assert.EqualValues(runtime.MachineStageResetting, machine.MachineStatusEvent_RESETTING)
	assert.EqualValues(runtime.MachineStageUpgrading, machine.MachineStatusEvent_UPGRADING)
}
