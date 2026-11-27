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
			Video:      hypervisor.VirtualMachineVideoSpec{Enabled: true},
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

func TestBuildDiskSources(t *testing.T) {
	platform, err := domain.PlatformFor(constants.ArchAmd64)
	require.NoError(t, err)

	empty := hypervisor.VirtualMachineDiskSpec{
		Name: "spare", Bus: "sata", Type: "cdrom",
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{Blank: true},
	}
	loaded := hypervisor.VirtualMachineDiskSpec{
		Name: "install", Pool: "pool1", Format: "raw", Bus: "sata", Type: "cdrom",
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{
			FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{Library: "lib", File: "talos.iso"},
		},
	}
	writable := hypervisor.VirtualMachineDiskSpec{
		Name: "system", Pool: "pool1", Format: "raw", Bus: "virtio", Type: "disk", Size: 1 << 30,
		Provision: hypervisor.VirtualMachineDiskProvisionSpec{Blank: true},
	}

	build := func(disk hypervisor.VirtualMachineDiskSpec, sourcePath string) domain.Inputs {
		inputs := testInputs(platform)
		inputs.SeedPath = ""
		inputs.Spec.Disks = []hypervisor.VirtualMachineDiskSpec{disk}
		inputs.Disks = map[string]hypervisor.VirtualMachineDiskStatusSpec{
			hypervisor.VirtualMachineDiskStatusID(inputs.Name, disk): {
				VirtualMachine: inputs.Name,
				Name:           disk.Name,
				SourcePath:     sourcePath,
				Format:         "raw",
				ReadOnly:       disk.Type == "cdrom",
				Phase:          hypervisor.VirtualMachineDiskPhaseReady,
			},
		}

		return inputs
	}

	for _, tc := range []struct {
		name   string
		disk   hypervisor.VirtualMachineDiskSpec
		source string
	}{
		{name: "empty drive", disk: empty},
		{name: "loaded drive", disk: loaded, source: "/lib/talos.iso"},
		{name: "writable disk", disk: writable, source: "/pool/system.raw"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			definition, _, err := domain.Build(build(tc.disk, tc.source))
			require.NoError(t, err)
			require.Len(t, definition.Devices.Disks, 1)

			rendered := definition.Devices.Disks[0]
			if tc.source == "" {
				require.Nil(t, rendered.Source)
			} else {
				require.Equal(t, tc.source, rendered.Source.File.File)
			}
		})
	}

	// A ready status without a source path is only valid for an empty drive.
	for _, disk := range []hypervisor.VirtualMachineDiskSpec{loaded, writable} {
		t.Run(disk.Name+" without source", func(t *testing.T) {
			_, _, err := domain.Build(build(disk, ""))
			require.ErrorIs(t, err, domain.ErrDiskNotReady)
		})
	}
}
