// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

// exampleDigest is a well-formed sha256 digest, used wherever a valid one is needed.
const exampleDigest = "sha256:5f2bc19e8b4b5b4a8b5e9c0d1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c"

//nolint:dupl
func TestVirtualMachineConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func() *hypervisor.VirtualMachineConfigV1Alpha1
	}{
		{
			name:     "ballooning",
			filename: "virtualmachineconfig.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm1"
				c.PowerStateConfig = hypervisorhelpers.PowerStateRunning
				c.CPUConfig.CPUCount = 4
				c.MemoryConfig.MemorySize = meta.MustByteSize("4GiB")
				c.MemoryConfig.BallooningConfig = &hypervisor.VirtualMachineBallooning{
					BallooningEnabled: new(true),
				}
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI

				return c
			},
		},
		{
			name:     "disks",
			filename: "virtualmachineconfig_disks.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm1"
				c.PowerStateConfig = hypervisorhelpers.PowerStateRunning
				c.CPUConfig.CPUCount = 4
				c.MemoryConfig.MemorySize = meta.MustByteSize("4GiB")
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI
				c.DisksConfig = []hypervisor.VirtualMachineDisk{
					{
						DiskName:      "system",
						DiskPool:      "pool1",
						DiskSize:      meta.MustByteSize("20GiB"),
						DiskFormat:    hypervisorhelpers.VirtualMachineDiskFormatQCOW2,
						DiskBus:       hypervisorhelpers.VirtualMachineDiskBusVirtio,
						DiskBootOrder: 1,
						ProvisionConfig: hypervisor.VirtualMachineDiskProvision{
							FromImageConfig: &hypervisor.VirtualMachineDiskFromImage{
								ImageLibrary: "images",
								ImageFile:    "talos-1.14.qcow2",
								ImageDigest:  exampleDigest,
								ImageMode:    hypervisorhelpers.VirtualMachineDiskImageModeLinked,
							},
						},
					},
					{
						DiskName: "data",
						DiskPool: "pool1",
						DiskSize: meta.MustByteSize("100GiB"),
						ProvisionConfig: hypervisor.VirtualMachineDiskProvision{
							BlankConfig: &hypervisor.VirtualMachineDiskBlank{},
						},
					},
					{
						DiskName:      "install",
						DiskPool:      "pool1",
						DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
						DiskBootOrder: 2,
						ProvisionConfig: hypervisor.VirtualMachineDiskProvision{
							FromImageConfig: &hypervisor.VirtualMachineDiskFromImage{
								ImageLibrary: "images",
								ImageFile:    "ubuntu-24.04.iso",
							},
						},
					},
				}

				return c
			},
		},
		{
			name:     "console",
			filename: "virtualmachineconfig_console.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm1"
				c.PowerStateConfig = hypervisorhelpers.PowerStateRunning
				c.CPUConfig.CPUCount = 4
				c.MemoryConfig.MemorySize = meta.MustByteSize("4GiB")
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI
				c.ConsoleConfig = hypervisor.VirtualMachineConsole{
					SerialConfig: hypervisor.VirtualMachineSerial{SerialEnabled: new(true)},
					VNCConfig:    hypervisor.VirtualMachineVNC{VNCEnabled: new(true)},
				}

				return c
			},
		},
		{
			name:     "secure boot",
			filename: "virtualmachineconfig_firmware.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm1"
				c.PowerStateConfig = hypervisorhelpers.PowerStateRunning
				c.CPUConfig.CPUCount = 4
				c.MemoryConfig.MemorySize = meta.MustByteSize("4GiB")
				c.FirmwareConfig = hypervisor.VirtualMachineFirmware{
					FirmwareType: hypervisorhelpers.VirtualMachineFirmwareTypeUEFI,
					SecureBootConfig: hypervisor.VirtualMachineFirmwareSecureBoot{
						SecureBootEnabled: new(true),
					},
				}

				return c
			},
		},
		{
			name:     "networking",
			filename: "virtualmachineconfig_networking.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm1"
				c.PowerStateConfig = hypervisorhelpers.PowerStateRunning
				c.CPUConfig.CPUCount = 4
				c.MemoryConfig.MemorySize = meta.MustByteSize("4GiB")
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI
				c.NetworkingConfig.InterfacesConfig = []hypervisor.VirtualMachineInterface{
					{
						InterfaceName: "net0",
						InterfaceLink: "eth0",
					},
					{
						InterfaceName: "net1",
						InterfaceLink: "eth1",
					},
				}

				return c
			},
		},
		{
			name:     "cpu limit",
			filename: "virtualmachineconfig_cpulimit.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm3"
				c.PowerStateConfig = hypervisorhelpers.PowerStateRunning
				c.CPUConfig.CPUCount = 4
				c.CPUConfig.CPULimit = "3000m"
				c.MemoryConfig.MemorySize = meta.MustByteSize("4GiB")
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI

				return c
			},
		},
		{
			name:     "minimal",
			filename: "virtualmachineconfig_minimal.yaml",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := hypervisor.NewVirtualMachineConfigV1Alpha1()
				c.MetaName = "vm2"
				c.PowerStateConfig = hypervisorhelpers.PowerStateRunning
				c.CPUConfig.CPUCount = 1
				c.MemoryConfig.MemorySize = meta.MustByteSize("512MiB")
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

			warnings, err := cfg.Validate(validationMode{})
			require.NoError(t, err)
			require.Empty(t, warnings)

			marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
			require.NoError(t, err)

			t.Log(string(marshaled))

			expectedMarshaled, err := os.ReadFile(filepath.Join("testdata", test.filename))
			require.NoError(t, err)

			assert.Equal(t, string(expectedMarshaled), string(marshaled))

			provider, err := configloader.NewFromBytes(expectedMarshaled)
			require.NoError(t, err)

			docs := provider.Documents()
			require.Len(t, docs, 1)

			assert.Equal(t, cfg, docs[0])
		})
	}
}

func TestVirtualMachineConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func() *hypervisor.VirtualMachineConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "empty",
			cfg:  hypervisor.NewVirtualMachineConfigV1Alpha1,

			expectedErrors: "name is required\npowerState is required\ncpu.count is required\nmemory.size is required\nfirmware.type is required",
		},
		{
			name: "invalid name",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MetaName = "my vm"

				return c
			},

			expectedErrors: `name "my vm": name can only contain ASCII letters, digits and hyphens`,
		},
		{
			name: "name too long",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MetaName = strings.Repeat("a", 64)

				return c
			},

			expectedErrors: fmt.Sprintf("name %q must be 63 characters or fewer", strings.Repeat("a", 64)),
		},
		{
			name: "no vCPUs",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPUCount = 0

				return c
			},

			expectedErrors: "cpu.count is required",
		},
		{
			name: "no memory",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MemoryConfig.MemorySize = meta.ByteSize{}

				return c
			},

			expectedErrors: "memory.size is required",
		},
		{
			name: "zero memory",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MemoryConfig.MemorySize = meta.MustByteSize("0")

				return c
			},

			expectedErrors: "memory.size must be greater than zero",
		},
		{
			name: "negative memory",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.MemoryConfig.MemorySize = meta.MustByteSize("-4GiB")

				return c
			},

			expectedErrors: "memory.size must not be negative",
		},
		{
			name: "disk without a pool or a source",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{
					{
						DiskName: "system",
						DiskSize: meta.MustByteSize("20GiB"),
					},
				}

				return c
			},

			expectedErrors: "disks[0]: pool is required\ndisks[0]: provision: exactly one of blank or fromImage must be set",
		},
		{
			name: "disk with both provisioning sources",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{blankDisk("system")}
				c.DisksConfig[0].ProvisionConfig.FromImageConfig = &hypervisor.VirtualMachineDiskFromImage{
					ImageLibrary: "images",
					ImageFile:    "talos.qcow2",
				}

				return c
			},

			expectedErrors: "disks[0]: provision: blank and fromImage are mutually exclusive",
		},
		{
			name: "duplicate disk name",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{blankDisk("system"), blankDisk("system")}

				return c
			},

			expectedErrors: `disks[1]: duplicate disk name "system"`,
		},
		{
			name: "duplicate disk name where one disk has another error",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{blankDisk("system"), blankDisk("system")}
				c.DisksConfig[0].DiskPool = ""

				return c
			},

			expectedErrors: "disks[0]: pool is required\ndisks[1]: duplicate disk name \"system\"",
		},
		{
			name: "duplicate bootOrder",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{blankDisk("system"), blankDisk("data")}
				c.DisksConfig[0].DiskBootOrder = 1
				c.DisksConfig[1].DiskBootOrder = 1

				return c
			},

			expectedErrors: "disks[1]: duplicate bootOrder 1",
		},
		{
			name: "linked clone of a raw disk",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{imageDisk("system")}
				c.DisksConfig[0].DiskFormat = hypervisorhelpers.VirtualMachineDiskFormatRaw
				c.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageMode = hypervisorhelpers.VirtualMachineDiskImageModeLinked

				return c
			},

			expectedErrors: "disks[0]: provision.fromImage.mode: linked requires format qcow2, as backing chains are a qcow2 feature",
		},
		{
			name: "linked clone on a cdrom",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{imageDisk("install")}
				c.DisksConfig[0].DiskType = hypervisorhelpers.VirtualMachineDiskTypeCDROM
				c.DisksConfig[0].DiskSize = meta.ByteSize{}
				c.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageMode = hypervisorhelpers.VirtualMachineDiskImageModeLinked

				return c
			},

			expectedErrors: "disks[0]: provision.fromImage.mode: linked is not allowed on a cdrom, which has no backing chain of its own",
		},
		{
			name: "sized cdrom",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{imageDisk("install")}
				c.DisksConfig[0].DiskType = hypervisorhelpers.VirtualMachineDiskTypeCDROM

				return c
			},

			expectedErrors: "disks[0]: size is not allowed on a cdrom",
		},
		{
			name: "virtio cdrom",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{imageDisk("install")}
				c.DisksConfig[0].DiskType = hypervisorhelpers.VirtualMachineDiskTypeCDROM
				c.DisksConfig[0].DiskSize = meta.ByteSize{}
				c.DisksConfig[0].DiskBus = hypervisorhelpers.VirtualMachineDiskBusVirtio

				return c
			},

			expectedErrors: "disks[0]: bus virtio is not allowed on a cdrom, which presents ejectable media",
		},
		{
			name: "blank cdrom",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{blankDisk("install")}
				c.DisksConfig[0].DiskType = hypervisorhelpers.VirtualMachineDiskTypeCDROM
				c.DisksConfig[0].DiskSize = meta.ByteSize{}

				return c
			},

			expectedErrors: "disks[0]: provision.blank: a cdrom has no contents of its own",
		},
		{
			name: "bus outside the enum",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{blankDisk("system")}
				c.DisksConfig[0].DiskBus = hypervisorhelpers.VirtualMachineDiskBus(99)

				return c
			},

			expectedErrors: `disks[0]: unsupported bus "VirtualMachineDiskBus(99)", expected virtio, scsi, sata or nvme`,
		},
		{
			name: "malformed digest",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{imageDisk("system")}
				c.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageDigest = "sha256:beef"

				return c
			},

			expectedErrors: `disks[0]: provision.fromImage.digest "sha256:beef" is invalid: invalid checksum digest length`,
		},
		{
			name: "sha512 digest",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{imageDisk("system")}
				c.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageDigest = "sha512:" + strings.Repeat("a", 128)

				return c
			},
		},
		{
			name: "unsupported digest algorithm",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{imageDisk("system")}
				c.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageDigest = "sha384:" + strings.Repeat("a", 96)

				return c
			},

			expectedErrors: `disks[0]: provision.fromImage.digest algorithm "sha384" is not supported, expected one of [sha256 sha512]`,
		},
		{
			name: "image file with a path separator",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{imageDisk("system")}
				c.DisksConfig[0].ProvisionConfig.FromImageConfig.ImageFile = "sub/talos.qcow2"

				return c
			},

			expectedErrors: `disks[0]: provision.fromImage.file "sub/talos.qcow2" must not contain a path separator`,
		},
		{
			name: "firmware type outside the enum",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareType(99)

				return c
			},

			expectedErrors: `unsupported firmware.type "VirtualMachineFirmwareType(99)", expected uefi or bios`,
		},
		{
			name: "secure boot on bios",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
				c.FirmwareConfig.SecureBootConfig = hypervisor.VirtualMachineFirmwareSecureBoot{
					SecureBootEnabled: new(true),
				}

				return c
			},

			expectedErrors: "firmware.secureBoot.enabled: secure boot requires firmware type uefi",
		},
		{
			name: "secure boot off on bios",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
				c.FirmwareConfig.SecureBootConfig = hypervisor.VirtualMachineFirmwareSecureBoot{
					SecureBootEnabled: new(false),
				}

				return c
			},
		},
		{
			name: "power state outside the enum",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.PowerStateConfig = hypervisorhelpers.PowerState(99)

				return c
			},

			expectedErrors: `unsupported powerState "PowerState(99)"`,
		},
		{
			name: "suspended",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.PowerStateConfig = hypervisorhelpers.PowerStateSuspended

				return c
			},
		},
		{
			name: "interface without a link",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.NetworkingConfig.InterfacesConfig = []hypervisor.VirtualMachineInterface{linkInterface("net0")}
				c.NetworkingConfig.InterfacesConfig[0].InterfaceLink = ""

				return c
			},

			expectedErrors: "networking.interfaces[0]: link is required",
		},
		{
			name: "link too long",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.NetworkingConfig.InterfacesConfig = []hypervisor.VirtualMachineInterface{linkInterface("net0")}
				c.NetworkingConfig.InterfacesConfig[0].InterfaceLink = "eth0123456789abc"

				return c
			},

			expectedErrors: "networking.interfaces[0]: link \"eth0123456789abc\" must not exceed 15 bytes",
		},
		{
			name: "link with a slash",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.NetworkingConfig.InterfacesConfig = []hypervisor.VirtualMachineInterface{linkInterface("net0")}
				c.NetworkingConfig.InterfacesConfig[0].InterfaceLink = "eth0/1"

				return c
			},

			expectedErrors: "networking.interfaces[0]: link \"eth0/1\" must not contain '/' or ':'",
		},
		{
			name: "link with whitespace",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.NetworkingConfig.InterfacesConfig = []hypervisor.VirtualMachineInterface{linkInterface("net0")}
				c.NetworkingConfig.InterfacesConfig[0].InterfaceLink = "eth 0"

				return c
			},

			expectedErrors: "networking.interfaces[0]: link \"eth 0\" must not contain whitespace or non-printable characters",
		},
		{
			name: "duplicate interface name",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.NetworkingConfig.InterfacesConfig = []hypervisor.VirtualMachineInterface{linkInterface("net0"), linkInterface("net0")}

				return c
			},

			expectedErrors: `networking.interfaces[1]: duplicate interface name "net0"`,
		},
		{
			name: "duplicate interface name after unrelated error",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.NetworkingConfig.InterfacesConfig = []hypervisor.VirtualMachineInterface{linkInterface("net0"), linkInterface("net0")}
				c.NetworkingConfig.InterfacesConfig[0].InterfaceLink = ""

				return c
			},

			expectedErrors: "networking.interfaces[0]: link is required\n" +
				"networking.interfaces[1]: duplicate interface name \"net0\"",
		},
		{
			name: "cpu limit without millicores",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPULimit = "2"

				return c
			},

			expectedErrors: `cpu.limit "2" must be expressed in millicores, e.g. 1500m`,
		},
		{
			name: "zero cpu limit",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPULimit = "0m"

				return c
			},

			expectedErrors: `cpu.limit "0m" must be greater than zero`,
		},
		{
			name: "cpu limit below minimum",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPULimit = "9m"

				return c
			},

			expectedErrors: "cpu.limit must be between 10 and 175921860444 millicores",
		},
		{
			name: "minimum cpu limit",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPULimit = "10m"

				return c
			},
		},
		{
			name: "maximum cpu limit",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPULimit = "175921860444m"

				return c
			},
		},
		{
			name: "cpu limit above maximum",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPULimit = "175921860445m"

				return c
			},

			expectedErrors: "cpu.limit must be between 10 and 175921860444 millicores",
		},
		{
			name: "missing cpu count with invalid cpu limit",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.CPUConfig.CPUCount = 0
				c.CPUConfig.CPULimit = "2"

				return c
			},

			expectedErrors: "cpu.count is required\n" +
				`cpu.limit "2" must be expressed in millicores, e.g. 1500m`,
		},
		{
			name: "valid",
			cfg:  validVirtualMachineConfig,
		},
		{
			name: "valid with interfaces",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.NetworkingConfig.InterfacesConfig = []hypervisor.VirtualMachineInterface{
					linkInterface("net0"), linkInterface("net1"), linkInterface("net2"),
				}

				return c
			},
		},
		{
			name: "valid with disks",
			cfg: func() *hypervisor.VirtualMachineConfigV1Alpha1 {
				c := validVirtualMachineConfig()
				c.DisksConfig = []hypervisor.VirtualMachineDisk{blankDisk("system"), imageDisk("data")}
				c.DisksConfig[0].DiskBootOrder = 1

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

			_, err := cfg.Validate(validationMode{})

			if test.expectedErrors == "" {
				require.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedErrors)
			}
		})
	}
}

func TestVirtualMachineDiskBusDefault(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		diskType hypervisorhelpers.VirtualMachineDiskType
		diskBus  hypervisorhelpers.VirtualMachineDiskBus

		expected hypervisorhelpers.VirtualMachineDiskBus
	}{
		{
			name:     "disk defaults to virtio",
			expected: hypervisorhelpers.VirtualMachineDiskBusVirtio,
		},
		{
			name:     "explicit disk type defaults to virtio",
			diskType: hypervisorhelpers.VirtualMachineDiskTypeDisk,
			expected: hypervisorhelpers.VirtualMachineDiskBusVirtio,
		},
		{
			name:     "cdrom defaults to sata, not virtio",
			diskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			expected: hypervisorhelpers.VirtualMachineDiskBusSATA,
		},
		{
			name:     "an explicit bus is left alone on a cdrom",
			diskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			diskBus:  hypervisorhelpers.VirtualMachineDiskBusSCSI,
			expected: hypervisorhelpers.VirtualMachineDiskBusSCSI,
		},
		{
			name:    "an explicit bus is left alone on a disk",
			diskBus: hypervisorhelpers.VirtualMachineDiskBusNVMe,

			expected: hypervisorhelpers.VirtualMachineDiskBusNVMe,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			disk := hypervisor.VirtualMachineDisk{
				DiskType: test.diskType,
				DiskBus:  test.diskBus,
			}

			assert.Equal(t, test.expected, disk.Bus())
		})
	}
}

func TestVirtualMachineConfigMergeDisks(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		base  hypervisor.VirtualMachineDiskList
		patch hypervisor.VirtualMachineDiskList

		expectedNames []string
		check         func(t *testing.T, disks hypervisor.VirtualMachineDiskList)
	}{
		{
			name:  "amends a disk by name",
			base:  hypervisor.VirtualMachineDiskList{blankDisk("system"), blankDisk("data")},
			patch: hypervisor.VirtualMachineDiskList{{DiskName: "data", DiskBootOrder: 2}},

			expectedNames: []string{"system", "data"},
			check: func(t *testing.T, disks hypervisor.VirtualMachineDiskList) {
				assert.Equal(t, uint32(2), disks[1].DiskBootOrder)
				assert.Equal(t, "pool1", disks[1].DiskPool)
				assert.NotNil(t, disks[1].ProvisionConfig.BlankConfig)
				assert.Zero(t, disks[0].DiskBootOrder)
			},
		},
		{
			name:  "appends a disk the base does not have",
			base:  hypervisor.VirtualMachineDiskList{blankDisk("system")},
			patch: hypervisor.VirtualMachineDiskList{blankDisk("scratch")},

			expectedNames: []string{"system", "scratch"},
		},
		{
			name:  "leaves the list alone when the patch omits disks",
			base:  hypervisor.VirtualMachineDiskList{blankDisk("system"), blankDisk("data")},
			patch: nil,

			expectedNames: []string{"system", "data"},
		},
		{
			name:  "merges a repeated name into a single entry",
			base:  hypervisor.VirtualMachineDiskList{blankDisk("system")},
			patch: hypervisor.VirtualMachineDiskList{{DiskName: "system", DiskPool: "pool2"}},

			expectedNames: []string{"system"},
			check: func(t *testing.T, disks hypervisor.VirtualMachineDiskList) {
				assert.Equal(t, "pool2", disks[0].DiskPool)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			base := validVirtualMachineConfig()
			base.DisksConfig = test.base

			patch := hypervisor.NewVirtualMachineConfigV1Alpha1()
			patch.MetaName = base.MetaName
			patch.DisksConfig = test.patch

			left, err := container.New(base)
			require.NoError(t, err)

			right, err := container.New(patch)
			require.NoError(t, err)

			merged, err := configpatcher.StrategicMerge(left, configpatcher.NewStrategicMergePatch(right))
			require.NoError(t, err)

			documents := merged.Documents()
			require.Len(t, documents, 1)

			cfg, ok := documents[0].(*hypervisor.VirtualMachineConfigV1Alpha1)
			require.True(t, ok)

			names := make([]string, 0, len(cfg.DisksConfig))
			for _, disk := range cfg.DisksConfig {
				names = append(names, disk.DiskName)
			}

			assert.Equal(t, test.expectedNames, names)

			// the patch must not carry the CPU and memory of the base away with it
			assert.Equal(t, uint32(4), cfg.CPUConfig.CPUCount)
			assert.Equal(t, uint64(4*1024*1024*1024), cfg.MemoryConfig.MemorySize.Value())

			if test.check != nil {
				test.check(t, cfg.DisksConfig)
			}
		})
	}
}

func TestVirtualMachineConfigClone(t *testing.T) {
	t.Parallel()

	c := validVirtualMachineConfig()
	c.MemoryConfig.BallooningConfig = &hypervisor.VirtualMachineBallooning{
		BallooningEnabled: new(true),
	}

	clone := c.Clone().(*hypervisor.VirtualMachineConfigV1Alpha1) //nolint:forcetypeassert,errcheck

	assert.Equal(t, c, clone)
	assert.NotSame(t, c, clone)
	assert.NotSame(t, c.MemoryConfig.BallooningConfig, clone.MemoryConfig.BallooningConfig)
	assert.NotSame(t, c.MemoryConfig.BallooningConfig.BallooningEnabled, clone.MemoryConfig.BallooningConfig.BallooningEnabled)

	// MemorySize keeps its buffers unexported, so the independence of the copy is
	// asserted by TestRegistryDeepCopyAudit in config/types, which can reach them.
}

func validVirtualMachineConfig() *hypervisor.VirtualMachineConfigV1Alpha1 {
	c := hypervisor.NewVirtualMachineConfigV1Alpha1()
	c.MetaName = "vm1"
	c.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	c.CPUConfig.CPUCount = 4
	c.MemoryConfig.MemorySize = meta.MustByteSize("4GiB")
	c.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI

	return c
}

func blankDisk(name string) hypervisor.VirtualMachineDisk {
	return hypervisor.VirtualMachineDisk{
		DiskName: name,
		DiskPool: "pool1",
		DiskSize: meta.MustByteSize("20GiB"),
		ProvisionConfig: hypervisor.VirtualMachineDiskProvision{
			BlankConfig: &hypervisor.VirtualMachineDiskBlank{},
		},
	}
}

func linkInterface(name string) hypervisor.VirtualMachineInterface {
	return hypervisor.VirtualMachineInterface{
		InterfaceName: name,
		InterfaceLink: "eth0",
	}
}

func imageDisk(name string) hypervisor.VirtualMachineDisk {
	return hypervisor.VirtualMachineDisk{
		DiskName: name,
		DiskPool: "pool1",
		DiskSize: meta.MustByteSize("20GiB"),
		ProvisionConfig: hypervisor.VirtualMachineDiskProvision{
			FromImageConfig: &hypervisor.VirtualMachineDiskFromImage{
				ImageLibrary: "images",
				ImageFile:    "talos.qcow2",
				ImageDigest:  exampleDigest,
			},
		},
	}
}

func TestVirtualMachineConfigConsoleDefaults(t *testing.T) {
	t.Parallel()

	t.Run("omitted console detaches both consoles", func(t *testing.T) {
		t.Parallel()

		cfg := hypervisor.NewVirtualMachineConfigV1Alpha1()

		assert.False(t, cfg.Console().Serial().Enabled())
		assert.False(t, cfg.Console().VNC().Enabled())
	})

	t.Run("serial can be turned on", func(t *testing.T) {
		t.Parallel()

		cfg := hypervisor.NewVirtualMachineConfigV1Alpha1()
		cfg.ConsoleConfig.SerialConfig = hypervisor.VirtualMachineSerial{SerialEnabled: new(true)}

		assert.True(t, cfg.Console().Serial().Enabled())
	})

	t.Run("vnc can be turned on", func(t *testing.T) {
		t.Parallel()

		cfg := hypervisor.NewVirtualMachineConfigV1Alpha1()
		cfg.ConsoleConfig.VNCConfig = hypervisor.VirtualMachineVNC{VNCEnabled: new(true)}

		assert.True(t, cfg.Console().VNC().Enabled())
	})
}

func TestVirtualMachineConfigFirmwareDefaults(t *testing.T) {
	t.Parallel()

	t.Run("omitted secureBoot reports disabled", func(t *testing.T) {
		t.Parallel()

		cfg := validVirtualMachineConfig()

		assert.Equal(t, hypervisorhelpers.VirtualMachineFirmwareTypeUEFI, cfg.Firmware().Type())
		assert.False(t, cfg.Firmware().SecureBoot().Enabled())
	})

	t.Run("secureBoot can be turned on", func(t *testing.T) {
		t.Parallel()

		cfg := validVirtualMachineConfig()
		cfg.FirmwareConfig.SecureBootConfig = hypervisor.VirtualMachineFirmwareSecureBoot{
			SecureBootEnabled: new(true),
		}

		assert.True(t, cfg.Firmware().SecureBoot().Enabled())
	})
}

// enumDoc is a valid document naming every enum-typed field, so that a case below can spoil one
// value at a time.
const enumDoc = `apiVersion: v1alpha1
kind: VirtualMachineConfig
name: vm1
powerState: running
cpu:
    count: 1
memory:
    size: 512MiB
firmware:
    type: uefi
disks:
    - name: system
      pool: pool1
      size: 20GiB
      format: qcow2
      bus: virtio
      type: disk
      provision:
        fromImage:
            library: images
            file: talos.qcow2
            mode: copy
`

// TestVirtualMachineConfigEnumUnmarshal checks that a value outside an enum is refused while the
// document is decoded, rather than carried into a struct and caught by validation later.
func TestVirtualMachineConfigEnumUnmarshal(t *testing.T) {
	t.Parallel()

	_, err := configloader.NewFromBytes([]byte(enumDoc))
	require.NoError(t, err, "the document every case below spoils must itself be valid")

	for _, test := range []struct {
		name     string
		valid    string
		spoiled  string
		expected string
	}{
		{
			name:     "power state",
			valid:    "powerState: running",
			spoiled:  "powerState: paused",
			expected: "paused does not belong to PowerState values",
		},
		{
			name:     "firmware type",
			valid:    "type: uefi",
			spoiled:  "type: seabios",
			expected: "seabios does not belong to VirtualMachineFirmwareType values",
		},
		{
			name:     "disk format",
			valid:    "format: qcow2",
			spoiled:  "format: vmdk",
			expected: "vmdk does not belong to VirtualMachineDiskFormat values",
		},
		{
			name:     "disk bus",
			valid:    "bus: virtio",
			spoiled:  "bus: ide",
			expected: "ide does not belong to VirtualMachineDiskBus values",
		},
		{
			name:     "disk type",
			valid:    "type: disk",
			spoiled:  "type: floppy",
			expected: "floppy does not belong to VirtualMachineDiskType values",
		},
		{
			name:     "image mode",
			valid:    "mode: copy",
			spoiled:  "mode: cow",
			expected: "cow does not belong to VirtualMachineDiskImageMode values",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := strings.Replace(enumDoc, test.valid, test.spoiled, 1)
			require.NotEqual(t, enumDoc, cfg, "%q not found in the document", test.valid)

			_, err := configloader.NewFromBytes([]byte(cfg))
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.expected)
		})
	}
}

// TestVirtualMachineConfigEnumDocValues guards the hand-written `values:` lists in the document
// against the enums they document: docgen copies those lists verbatim, so nothing else notices
// when a member is added to hypervisorhelpers and not to the comment. The zero member of each enum
// stands for "unset" and is not a value a document may name, so it is not documented either.
func TestVirtualMachineConfigEnumDocValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		doc      *encoder.Doc
		field    string
		expected []string
	}{
		{
			name:     "power state",
			doc:      hypervisor.VirtualMachineConfigV1Alpha1{}.Doc(),
			field:    "powerState",
			expected: hypervisorhelpers.NameableValues(hypervisorhelpers.PowerStateStrings()),
		},
		{
			name:     "firmware type",
			doc:      hypervisor.VirtualMachineFirmware{}.Doc(),
			field:    "type",
			expected: hypervisorhelpers.NameableValues(hypervisorhelpers.VirtualMachineFirmwareTypeStrings()),
		},
		{
			name:     "disk format",
			doc:      hypervisor.VirtualMachineDisk{}.Doc(),
			field:    "format",
			expected: hypervisorhelpers.NameableValues(hypervisorhelpers.VirtualMachineDiskFormatStrings()),
		},
		{
			name:     "disk bus",
			doc:      hypervisor.VirtualMachineDisk{}.Doc(),
			field:    "bus",
			expected: hypervisorhelpers.NameableValues(hypervisorhelpers.VirtualMachineDiskBusStrings()),
		},
		{
			name:     "disk type",
			doc:      hypervisor.VirtualMachineDisk{}.Doc(),
			field:    "type",
			expected: hypervisorhelpers.NameableValues(hypervisorhelpers.VirtualMachineDiskTypeStrings()),
		},
		{
			name:     "image mode",
			doc:      hypervisor.VirtualMachineDiskFromImage{}.Doc(),
			field:    "mode",
			expected: hypervisorhelpers.NameableValues(hypervisorhelpers.VirtualMachineDiskImageModeStrings()),
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			idx := slices.IndexFunc(test.doc.Fields, func(field encoder.Doc) bool { return field.Name == test.field })
			require.GreaterOrEqual(t, idx, 0, "%s field not found in the document's docs", test.field)

			assert.Equal(t, test.expected, test.doc.Fields[idx].Values)
		})
	}
}
