// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package profile_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/profile"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestARM64Profiles(t *testing.T) {
	builtins := profile.Profiles(constants.ArchArm64)
	require.Len(t, builtins, 2)

	for index, name := range []string{"host", "passthrough"} {
		builtin := builtins[index]
		require.Equal(t, name, builtin.Metadata().ID())
		require.Equal(t, hypervisor.VirtualMachineProfileSpecSpec{
			Builtin: true,
			Machine: "virt",
			CPU: hypervisor.VirtualMachineProfileCPUSpec{
				Mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
			},
		}, *builtin.TypedSpec())
		require.NoError(t, profile.Validate(constants.ArchArm64, name, *builtin.TypedSpec()))
	}

	require.Error(t, profile.Validate(constants.ArchArm64, "legacy", *builtins[0].TypedSpec()))
}

func TestARM64ValidateCustom(t *testing.T) {
	spec := hypervisor.VirtualMachineProfileSpecSpec{
		Machine: "virt-9.2",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough,
		},
		Timers: hypervisor.DefaultVirtualMachineProfileTimersSpec(),
	}
	require.NoError(t, profile.ValidateCustom(constants.ArchArm64, "cluster", spec))

	spec.Timers.HPET = true
	require.Error(t, profile.ValidateCustom(constants.ArchArm64, "cluster", spec))
	spec.Timers.HPET = false

	spec.Machine = "q35"
	require.Error(t, profile.ValidateCustom(constants.ArchArm64, "cluster", spec))
}
