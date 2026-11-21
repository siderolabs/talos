// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package domain_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/domain"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func testInputs(platform domain.Platform) domain.Inputs {
	return domain.Inputs{
		Platform: platform,
		Name:     "guest",
		Spec: &hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024 * 1024},
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Console:    hypervisor.VirtualMachineConsoleSpec{Serial: true, VNC: true},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{{Name: "net0", Attachment: "bridge"}},
		},
		Profile:  &hypervisor.VirtualMachineProfileSpecSpec{Machine: "virt", CPU: hypervisor.VirtualMachineProfileCPUSpec{Mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough}},
		Links:    []string{"br0"},
		SeedPath: "/pool/seed.iso",
	}
}

func TestBuildSeedAndResolvedHardware(t *testing.T) {
	for _, arch := range []string{constants.ArchAmd64, constants.ArchArm64} {
		t.Run(arch, func(t *testing.T) {
			platform, err := domain.PlatformFor(arch)
			require.NoError(t, err)

			definition, disks, err := domain.Build(testInputs(platform))
			require.NoError(t, err)
			require.Empty(t, disks)
			require.Equal(t, "efi", definition.OS.Firmware)
			require.NotNil(t, definition.Features.ACPI)
			require.Len(t, definition.Devices.Interfaces, 1)
			require.Equal(t, "br0", definition.Devices.Interfaces[0].Source.Bridge.Bridge)
			require.Len(t, definition.Devices.Channels, 1)
			require.Len(t, definition.Devices.Serials, 1)
			require.Len(t, definition.Devices.Disks, 1)
			seed := definition.Devices.Disks[0]
			require.Equal(t, "cdrom", seed.Device)
			require.NotNil(t, seed.ReadOnly)
			require.Nil(t, seed.Boot)
			require.Equal(t, "/pool/seed.iso", seed.Source.File.File)

			_, err = definition.Marshal()
			require.NoError(t, err)
		})
	}
}

func TestPlatformFor(t *testing.T) {
	_, err := domain.PlatformFor("riscv64")
	require.ErrorContains(t, err, `architecture "riscv64" is unsupported`)

	amd64, err := domain.PlatformFor(constants.ArchAmd64)
	require.NoError(t, err)
	require.False(t, amd64.RequiresUEFI())

	arm64, err := domain.PlatformFor(constants.ArchArm64)
	require.NoError(t, err)
	require.True(t, arm64.RequiresUEFI())
}
