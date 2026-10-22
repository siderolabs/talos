// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

type VirtualMachineProjectionSuite struct {
	ctest.DefaultSuite
}

func TestVirtualMachineProjectionSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VirtualMachineProjectionSuite{
		Timeout: 30 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineSpecController{}))
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineProfileSpecController{}))
		},
	})
}

func (suite *VirtualMachineProjectionSuite) TestProjectsProfileDefaultAndNamedReference() {
	cfg, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: VirtualMachineConfig
name: omitted
powerState: running
cpu: {count: 1}
memory: {size: 1GiB}
firmware: {type: uefi}
---
apiVersion: v1alpha1
kind: VirtualMachineConfig
name: named
profile: rack-a
powerState: running
cpu: {count: 1}
memory: {size: 1GiB}
firmware: {type: uefi}
`))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	ctest.AssertResource(suite, "omitted", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal("host", res.TypedSpec().Profile)
	})
	ctest.AssertResource(suite, "named", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal("rack-a", res.TypedSpec().Profile)
	})
}

func (suite *VirtualMachineProjectionSuite) TestProfileCatalogueAndCleanup() {
	if runtime.GOARCH != "amd64" {
		suite.T().Skip("builtin VM profiles are only supported on amd64")
	}

	ctest.AssertResource(suite, "host", func(res *hypervisor.VirtualMachineProfileSpec, asrt *assert.Assertions) {
		asrt.True(res.TypedSpec().Builtin)
		asrt.Equal("q35", res.TypedSpec().Machine)
		asrt.Equal(hypervisorhelpers.VirtualMachineProfileCPUModeHostModel, res.TypedSpec().CPU.Mode)
	})
	ctest.AssertResource(suite, "legacy", func(res *hypervisor.VirtualMachineProfileSpec, asrt *assert.Assertions) {
		asrt.True(res.TypedSpec().Builtin)
		asrt.Equal("pc-i440fx-9.2", res.TypedSpec().Machine)
		asrt.Equal(hypervisorhelpers.VirtualMachineProfileCPUModeCustom, res.TypedSpec().CPU.Mode)
		asrt.Equal("kvm64", res.TypedSpec().CPU.Model)
		asrt.Equal("forbid", res.TypedSpec().CPU.Fallback)
		asrt.Empty(res.TypedSpec().CPU.Features)
	})
	ctest.AssertResource(suite, "passthrough", func(res *hypervisor.VirtualMachineProfileSpec, asrt *assert.Assertions) {
		asrt.True(res.TypedSpec().Builtin)
		asrt.Equal(hypervisorhelpers.VirtualMachineProfileCPUModeHostPassthrough, res.TypedSpec().CPU.Mode)
	})

	doc := hypervisorcfg.NewVirtualMachineProfileConfigV1Alpha1()
	doc.MetaName = "rack-a"
	doc.MachineConfig = "pc-q35-9.2"
	doc.CPUConfig.ModeConfig = hypervisorhelpers.VirtualMachineProfileCPUModeCustom
	doc.CPUConfig.ModelConfig = "x86-64-v3"
	cfg, err := container.New(doc)
	suite.Require().NoError(err)

	machine := config.NewMachineConfig(cfg)
	suite.Create(machine)
	ctest.AssertResource(suite, "rack-a", func(res *hypervisor.VirtualMachineProfileSpec, asrt *assert.Assertions) {
		asrt.False(res.TypedSpec().Builtin)
		asrt.Equal("pc-q35-9.2", res.TypedSpec().Machine)
		asrt.Equal("x86-64-v3", res.TypedSpec().CPU.Model)
	})
	suite.Destroy(machine)
	ctest.AssertNoResource[*hypervisor.VirtualMachineProfileSpec](suite, "rack-a")
	ctest.AssertResource(suite, "host", func(res *hypervisor.VirtualMachineProfileSpec, asrt *assert.Assertions) {
		asrt.True(res.TypedSpec().Builtin)
	})
}

func (suite *VirtualMachineProjectionSuite) TestProjectsUpdatesAndRemovesTypedSpecs() {
	doc := newVirtualMachine("guest-one")
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal(
			hypervisor.VirtualMachineSpecSpec{
				Profile: "host",
				CPU: hypervisor.VirtualMachineCPUSpec{
					Count: 3,
				},
				Memory: hypervisor.VirtualMachineMemorySpec{
					Size: 4 << 30,
				},
				PowerState: "running",
				Firmware: hypervisor.VirtualMachineFirmwareSpec{
					Type: "uefi",
				},
				Guest: hypervisor.VirtualMachineGuestSpec{
					Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true},
				},
			},
			*res.TypedSpec(),
		)
		asrt.Equal("hypervisor.VirtualMachineSpecController", res.Metadata().Owner())
	})

	// Backend representability belongs to the renderer, not this projection.
	doc.CPUConfig.CPUCount = 65536
	doc.MemoryConfig.MemorySize = meta.MustByteSize("1025")
	doc.MemoryConfig.BallooningConfig = &hypervisorcfg.VirtualMachineBallooning{
		BallooningEnabled: new(true),
	}
	cfg, err = container.New(doc)
	suite.Require().NoError(err)

	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)

	replacement := config.NewMachineConfig(cfg)
	replacement.Metadata().SetVersion(old.Metadata().Version())
	suite.Update(replacement)
	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal(
			hypervisor.VirtualMachineSpecSpec{
				Profile: "host",
				CPU: hypervisor.VirtualMachineCPUSpec{
					Count: 65536,
				},
				Memory: hypervisor.VirtualMachineMemorySpec{
					Size: 1025,
					Ballooning: hypervisor.VirtualMachineMemoryBallooningSpec{
						Enabled: true,
					},
				},
				PowerState: "running",
				Firmware: hypervisor.VirtualMachineFirmwareSpec{
					Type: "uefi",
				},
				Guest: hypervisor.VirtualMachineGuestSpec{
					Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true},
				},
			},
			*res.TypedSpec(),
		)
	})

	suite.Destroy(replacement)
	ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, doc.Name())
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
}

type VirtualMachineSpecSuite struct {
	ctest.DefaultSuite
	logs *observer.ObservedLogs
}

func TestVirtualMachineSpecSuite(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zap.DebugLevel)
	suite.Run(t, &VirtualMachineSpecSuite{
		Timeout: 30 * time.Second,
		Logger:  zap.New(core),
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineSpecController{}))
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDiskController{}))
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineProfileSpecController{}))
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
		},
		logs: logs,
	})
}

func newVirtualMachine(name string) *hypervisorcfg.VirtualMachineConfigV1Alpha1 {
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI
	doc.CPUConfig.CPUCount = 3
	doc.MemoryConfig.MemorySize = meta.MustByteSize("4GiB")

	return doc
}

// A power override wins over the configured power state, whatever the configuration is changed
// to, and dropping it gives the configured power state back.
func (suite *VirtualMachineProjectionSuite) TestPowerOverrideWinsOverConfiguredPowerState() {
	doc := newVirtualMachine("overridden")
	cfg, err := container.New(doc)
	suite.Require().NoError(err)

	active := config.NewMachineConfig(cfg)
	suite.Create(active)

	override := hypervisor.NewVirtualMachinePowerOverride(hypervisor.NamespaceName, doc.Name())
	override.TypedSpec().PowerState = "stopped"
	override.TypedSpec().StopRequest = "graceful/request-1"
	suite.Create(override)

	ctest.AssertResource(suite, doc.Name(), func(spec *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal("running", spec.TypedSpec().PowerState)
	})

	edited := newVirtualMachine("overridden")
	edited.CPUConfig.CPUCount = 2
	edited.PowerStateConfig = hypervisorhelpers.PowerStateSuspended
	cfg, err = container.New(edited)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), active.Metadata()))
	suite.Create(config.NewMachineConfig(cfg))

	ctest.AssertResource(suite, doc.Name(), func(spec *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.EqualValues(2, spec.TypedSpec().CPU.Count)
		asrt.Equal("suspended", spec.TypedSpec().PowerState)
	})

	suite.Destroy(override)

	ctest.AssertResource(suite, doc.Name(), func(spec *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal("suspended", spec.TypedSpec().PowerState)
	})
}

func (suite *VirtualMachineProjectionSuite) TestProjectsRequiredAndOptionalIntent() {
	doc := newVirtualMachine("intent")
	doc.PowerStateConfig = hypervisorhelpers.PowerStateSuspended
	doc.FirmwareConfig.SecureBootConfig.SecureBootEnabled = new(true)
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	doc.ConsoleConfig.VNCConfig.VNCEnabled = new(true)
	doc.CPUConfig.CPULimit = "2500m"
	doc.CPUConfig.TopologyConfig.TopologySockets = new(uint32(1))
	doc.CPUConfig.TopologyConfig.TopologyCores = new(uint32(3))
	doc.CPUConfig.TopologyConfig.TopologyThreads = new(uint32(1))
	doc.CPUConfig.TopologyConfig.PinningConfig = hypervisorcfg.VirtualMachineCPUPinning{
		// Written out of order and with a duplicate: the projection carries the canonical form.
		VCPUsConfig:    []hypervisorcfg.VirtualMachineVCPUPin{{PinVCPU: 2, PinCPUs: "9,8,9"}, {PinVCPU: 0, PinCPUs: "1"}},
		EmulatorConfig: "0-1",
	}
	doc.MemoryConfig.NUMAConfig = &hypervisorcfg.VirtualMachineNUMA{NUMANodes: "1"}
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "system",
			DiskPool:      "pool1",
			DiskSize:      meta.MustByteSize("20GiB"),
			DiskBootOrder: 1,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "images",
					ImageFile:    "system.qcow2",
					ImageMode:    hypervisorhelpers.VirtualMachineDiskImageModeLinked,
				},
			},
		},
	}
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "eth0"},
		{InterfaceName: "net1", InterfaceLink: "uplink"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineSpecSpec{
			Profile: "host",
			CPU: hypervisor.VirtualMachineCPUSpec{
				Count:       3,
				Limit:       2500,
				Pins:        []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 2, CPUs: "8-9"}, {VCPU: 0, CPUs: "1"}},
				EmulatorPin: "0-1",
				Topology:    &hypervisor.VirtualMachineCPUTopologySpec{Sockets: 1, Cores: 3, Threads: 1},
			},
			Memory: hypervisor.VirtualMachineMemorySpec{
				Size: 4 << 30,
				NUMA: &hypervisor.VirtualMachineMemoryNUMASpec{Mode: "strict", Nodes: "1"},
			},
			PowerState: "suspended",

			Firmware: hypervisor.VirtualMachineFirmwareSpec{
				Type:       "uefi",
				SecureBoot: true,
			},
			Console: hypervisor.VirtualMachineConsoleSpec{
				Serial: true,
				VNC:    true,
			},
			Disks: []hypervisor.VirtualMachineDiskSpec{
				{
					Name:      "system",
					Pool:      "pool1",
					Size:      20 << 30,
					Format:    "qcow2",
					Bus:       "virtio",
					Type:      "disk",
					BootOrder: 1,
					Provision: hypervisor.VirtualMachineDiskProvisionSpec{
						FromImage: &hypervisor.VirtualMachineDiskFromImageSpec{
							Library: "images",
							File:    "system.qcow2",
							Mode:    "linked",
						},
					},
				},
			},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{
				{Name: "net0", Link: "eth0"},
				{Name: "net1", Link: "uplink"},
			},
			Guest: hypervisor.VirtualMachineGuestSpec{
				Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true},
			},
		}, *res.TypedSpec())
	})
}

// Compare the complete emitted definition, including absent devices and defaults.
// Validate the same controller-produced XML against libvirt's Relax NG schema.
func (suite *VirtualMachineSpecSuite) assertDomain(name, fixture string) {
	suite.T().Helper()

	want, err := os.ReadFile(filepath.Join("testdata", "virtualmachinespec", fixture+".xml"))
	suite.Require().NoError(err)

	ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal(string(want), res.TypedSpec().DomainXML+"\n")
		asrt.Equal(hypervisor.NamespaceName, res.Metadata().Namespace())
		asrt.Equal("hypervisor.VirtualMachineDomainSpecController", res.Metadata().Owner())
	})

	res, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), name)
	suite.Require().NoError(err)

	suite.Require().NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
}

func (suite *VirtualMachineSpecSuite) TestFirmwareAndConsoles() {
	bios := newVirtualMachine("bios")
	bios.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	uefi := newVirtualMachine("uefi")
	uefi.FirmwareConfig.SecureBootConfig.SecureBootEnabled = new(true)
	uefi.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	uefi.ConsoleConfig.VNCConfig.VNCEnabled = new(true)
	cfg, err := container.New(bios, uefi)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, "bios", func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, "<os>")
		asrt.NotContains(res.TypedSpec().DomainXML, `firmware="bios"`)
	})
	ctest.AssertResource(suite, "uefi", func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, `<os firmware="efi">`)
		asrt.Contains(res.TypedSpec().DomainXML, `name="secure-boot"`)
		asrt.Contains(res.TypedSpec().DomainXML, `<serial type="pty">`)
		asrt.Contains(res.TypedSpec().DomainXML, `<graphics type="vnc"`)
	})
}

func (suite *VirtualMachineSpecSuite) TestGuestAgentChannel() {
	defaultOn := newVirtualMachine("agent-default")
	explicitOff := newVirtualMachine("agent-off")
	explicitOff.GuestConfig.AgentConfig = &hypervisorcfg.VirtualMachineAgent{AgentEnabled: new(false)}
	cfg, err := container.New(defaultOn, explicitOff)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	// The channel is attached by default: the operator should not have to flip anything to
	// benefit from guest-agent features when the agent is installed inside the guest.
	ctest.AssertResource(suite, "agent-default", func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, `<channel type="unix">`)
		asrt.Contains(res.TypedSpec().DomainXML, `<target type="virtio" name="org.qemu.guest_agent.0">`)
	})
	ctest.AssertResource(suite, "agent-off", func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.NotContains(res.TypedSpec().DomainXML, "org.qemu.guest_agent.0")
	})
}

// Copying a disk from a content library image is not implemented, so a virtual machine asking for
// one never gets a domain, and says so.
func (suite *VirtualMachineSpecSuite) TestRejectsUnsupportedDisks() {
	doc := newVirtualMachine("unsupported")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{{
		DiskName: "system", DiskPool: "pool1", DiskSize: meta.MustByteSize("20GiB"),
		ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
			FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{ImageLibrary: "images", ImageFile: "talos.qcow2"},
		},
	}}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal("pool1", res.TypedSpec().Disks[0].Pool)
	})
	suite.assertConversionError(doc.Name(), `disk "system": unsupported disk: a disk is only provisioned from provision.blank today`)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
}

func (suite *VirtualMachineSpecSuite) TestPublishesDomainXML() {
	cfg, err := container.New(newVirtualMachine("guest-one"))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertDomain("guest-one", "default")
}

func (suite *VirtualMachineSpecSuite) TestIntegrationXMLFixtures() {
	created := newVirtualMachine("vm-integration")
	created.MemoryConfig.BallooningConfig = &hypervisorcfg.VirtualMachineBallooning{
		BallooningEnabled: new(true),
	}
	created.CPUConfig.CPULimit = "2500m"
	updated := newVirtualMachine("vm-integration-updated")
	updated.CPUConfig.CPUCount = 1
	updated.MemoryConfig.MemorySize = meta.MustByteSize("512MiB")

	cfg, err := container.New(created, updated)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	for _, tc := range []struct {
		name    string
		fixture string
	}{
		{name: created.Name(), fixture: "created"},
		{name: updated.Name(), fixture: "updated"},
	} {
		want, readErr := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "integration", "api", "testdata", "virtualmachinespec", tc.fixture+".xml"))
		suite.Require().NoError(readErr)
		ctest.AssertResource(suite, tc.name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Equal(strings.Replace(string(want), "<name>vm-integration</name>", "<name>"+tc.name+"</name>", 1), res.TypedSpec().DomainXML+"\n")
		})
	}
}

func (suite *VirtualMachineSpecSuite) TestRejectsUnalignedMemory() {
	for _, size := range []string{"1", "1023", "1025", "4294967297", "9223372036854774783"} {
		suite.logs.TakeAll()

		doc := newVirtualMachine("unaligned-" + size)
		doc.MemoryConfig.MemorySize = meta.MustByteSize(size)
		// Valid Talos memory values that libvirt would silently round up to KiB.
		suite.Require().NoError(doc.ValidateMemory())

		cfg, err := container.New(doc)
		suite.Require().NoError(err)
		suite.Create(config.NewMachineConfig(cfg))
		suite.assertConversionError(doc.Name(), "memory size must be a multiple of 1024 bytes")
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
		suite.Destroy(config.NewMachineConfig(cfg))
	}
}

func (suite *VirtualMachineSpecSuite) assertConversionError(name, reason string) {
	suite.T().Helper()
	suite.Require().Eventually(func() bool {
		return suite.logs.Filter(func(entry observer.LoggedEntry) bool {
			err, ok := entry.ContextMap()["error"].(string)

			return ok && strings.Contains(err, fmt.Sprintf("virtual machine %q: %s", name, reason))
		}).Len() > 0
	}, 30*time.Second, time.Millisecond)
}

func (suite *VirtualMachineSpecSuite) TestRejectsCPUOutsideSchemaRange() {
	for _, count := range []uint32{0, 65536, 4294967295} {
		suite.logs.TakeAll()

		doc := newVirtualMachine(fmt.Sprintf("invalid-cpu-%d", count))

		doc.CPUConfig.CPUCount = count
		if count != 0 {
			suite.Require().NoError(doc.ValidateCPU())
		}

		cfg, err := container.New(doc)
		suite.Require().NoError(err)
		suite.Create(config.NewMachineConfig(cfg))
		suite.assertConversionError(doc.Name(), "")
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
		suite.Destroy(config.NewMachineConfig(cfg))
	}
}

func (suite *VirtualMachineSpecSuite) TestRejectsMemoryOutsideParserRange() {
	for _, size := range []string{"0", "9223372036854774785", "9223372036854775808", "18446744073709551615"} {
		suite.logs.TakeAll()

		doc := newVirtualMachine("invalid-memory-" + size)

		doc.MemoryConfig.MemorySize = meta.MustByteSize(size)
		if size != "0" {
			suite.Require().NoError(doc.ValidateMemory())
		}

		cfg, err := container.New(doc)
		suite.Require().NoError(err)
		suite.Create(config.NewMachineConfig(cfg))
		suite.assertConversionError(doc.Name(), "")
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
		suite.Destroy(config.NewMachineConfig(cfg))
	}
}

func (suite *VirtualMachineSpecSuite) TestRepresentableBoundaries() {
	minimum := newVirtualMachine("minimum")
	minimum.CPUConfig.CPUCount = 1
	minimum.MemoryConfig.MemorySize = meta.MustByteSize("1024")
	maximum := newVirtualMachine("maximum")
	maximum.CPUConfig.CPUCount = 65535
	maximum.MemoryConfig.MemorySize = meta.MustByteSize("9223372036854774784")

	cfg, err := container.New(minimum, maximum)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertDomain(minimum.Name(), "minimum")
	suite.assertDomain(maximum.Name(), "maximum")
}

func (suite *VirtualMachineSpecSuite) TestInvalidUpdatePreservesLastGoodSpecAndRecovers() {
	doc := newVirtualMachine("guest-one")
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertDomain(doc.Name(), "default")
	suite.logs.TakeAll()

	doc.MemoryConfig.MemorySize = meta.MustByteSize("1073741825")
	suite.replaceConfig(doc)
	suite.assertConversionError(doc.Name(), "memory size must be a multiple of 1024 bytes")
	suite.assertDomain(doc.Name(), "default")

	suite.replaceConfig(newVirtualMachine("balloon"))
	suite.assertDomain("balloon", "balloon-disabled")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
}

func (suite *VirtualMachineSpecSuite) TestDeterministicProjection() {
	doc := newVirtualMachine("stable")
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, doc.Name(), func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	before, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), doc.Name())
	suite.Require().NoError(err)

	// A second output is a reconciliation barrier for the no-op version check.
	doc = newVirtualMachine("stable")
	doc.MemoryConfig.BallooningConfig = &hypervisorcfg.VirtualMachineBallooning{BallooningEnabled: new(false)}
	suite.replaceConfig(doc, newVirtualMachine("barrier"))
	ctest.AssertResource(suite, "barrier", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	after, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), doc.Name())
	suite.Require().NoError(err)
	suite.Equal(before.TypedSpec().DomainXML, after.TypedSpec().DomainXML)
	suite.Equal(before.Metadata().Version(), after.Metadata().Version())
}

func (suite *VirtualMachineSpecSuite) TestIgnoresPersistentConfigButRejectsUnsupportedDisks() {
	persistent, err := container.New(newVirtualMachine("staged-only"))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfigWithID(persistent, config.PersistentID))

	doc := newVirtualMachine("active-only")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName: "deferred",
			DiskPool: "unresolved",
			DiskSize: meta.MustByteSize("20GiB"),
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{ImageLibrary: "images", ImageFile: "talos.qcow2"},
			},
		},
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertConversionError(doc.Name(), `disk "deferred": unsupported disk: a disk is only provisioned from provision.blank today`)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "staged-only")
}

func (suite *VirtualMachineSpecSuite) TestRemovesSpecsWhenConfigDisappears() {
	doc := newVirtualMachine("removed")
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, doc.Name(), func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})

	suite.Destroy(config.NewMachineConfig(cfg))
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
}

func (suite *VirtualMachineSpecSuite) TestRemovesUnconfiguredSpecs() {
	first := newVirtualMachine("first")
	second := newVirtualMachine("second")
	cfg, err := container.New(first, second)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, "first", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	ctest.AssertResource(suite, "second", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})

	suite.replaceConfig(second)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "first")
	ctest.AssertResource(suite, "second", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	suite.replaceConfig()
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "second")
}

func (suite *VirtualMachineSpecSuite) TestBallooningUpdates() {
	doc := newVirtualMachine("balloon")
	doc.MemoryConfig.BallooningConfig = &hypervisorcfg.VirtualMachineBallooning{BallooningEnabled: new(true)}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertDomain(doc.Name(), "balloon-enabled")

	doc = newVirtualMachine("balloon")
	doc.CPUConfig.CPUCount = 7
	doc.MemoryConfig.MemorySize = meta.MustByteSize("1GiB")
	suite.replaceConfig(doc)
	suite.assertDomain(doc.Name(), "balloon-omitted")

	doc.MemoryConfig.BallooningConfig = &hypervisorcfg.VirtualMachineBallooning{BallooningEnabled: new(false)}
	suite.replaceConfig(doc, newVirtualMachine("barrier"))
	ctest.AssertResource(suite, "barrier", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	suite.assertDomain(doc.Name(), "balloon-omitted")

	doc.MemoryConfig.BallooningConfig = &hypervisorcfg.VirtualMachineBallooning{}
	suite.replaceConfig(doc, newVirtualMachine("empty-balloon-barrier"))
	ctest.AssertResource(suite, "empty-balloon-barrier", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	suite.assertDomain(doc.Name(), "balloon-omitted")
}

func (suite *VirtualMachineSpecSuite) TestCPULimit() {
	doc := newVirtualMachine("guest-one")
	doc.CPUConfig.CPULimit = "2500m"
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertDomain(doc.Name(), "cpu-limit")

	// Dropping the limit must drop cputune, not leave the last quota behind.
	doc.CPUConfig.CPULimit = ""
	suite.replaceConfig(doc)
	suite.assertDomain(doc.Name(), "default")
}

func (suite *VirtualMachineSpecSuite) TestCPUPinningAndNUMA() {
	doc := newVirtualMachine("guest-one")
	doc.CPUConfig.TopologyConfig.PinningConfig = hypervisorcfg.VirtualMachineCPUPinning{
		VCPUsConfig:    []hypervisorcfg.VirtualMachineVCPUPin{{PinVCPU: 0, PinCPUs: "8"}, {PinVCPU: 1, PinCPUs: "9-10"}},
		EmulatorConfig: "0-1",
	}
	doc.MemoryConfig.NUMAConfig = &hypervisorcfg.VirtualMachineNUMA{NUMANodes: "1"}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertDomain(doc.Name(), "cpu-pinning")

	// The quota and the pins share one cputune; a changed pin set must not leave the old pins behind.
	doc.CPUConfig.CPULimit = "2500m"
	doc.CPUConfig.TopologyConfig.PinningConfig = hypervisorcfg.VirtualMachineCPUPinning{
		VCPUsConfig: []hypervisorcfg.VirtualMachineVCPUPin{{PinVCPU: 2, PinCPUs: "4"}},
	}
	doc.MemoryConfig.NUMAConfig = &hypervisorcfg.VirtualMachineNUMA{
		NUMAMode:  hypervisorhelpers.VirtualMachineNUMAModeInterleave,
		NUMANodes: "0-1",
	}
	suite.replaceConfig(doc)
	suite.assertDomain(doc.Name(), "cpu-pinning-limit")

	// Dropping the pins and the placement leaves the quota alone, and dropping that too drops cputune.
	doc.CPUConfig.TopologyConfig.PinningConfig = hypervisorcfg.VirtualMachineCPUPinning{}
	doc.MemoryConfig.NUMAConfig = nil
	suite.replaceConfig(doc)
	suite.assertDomain(doc.Name(), "cpu-limit")

	doc.CPUConfig.CPULimit = ""
	suite.replaceConfig(doc)
	suite.assertDomain(doc.Name(), "default")
}

func (suite *VirtualMachineSpecSuite) TestCPUTopologyLifecycle() {
	doc := newVirtualMachine("guest-one")
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertDomain(doc.Name(), "default")

	for _, topology := range []hypervisor.VirtualMachineCPUTopologySpec{
		{Sockets: 3, Cores: 1, Threads: 1},
		{Sockets: 1, Cores: 2, Threads: 2},
	} {
		doc.CPUConfig.CPUCount = topology.Sockets * topology.Cores * topology.Threads
		doc.CPUConfig.TopologyConfig = hypervisorcfg.VirtualMachineCPUTopology{
			TopologySockets: new(topology.Sockets),
			TopologyCores:   new(topology.Cores),
			TopologyThreads: new(topology.Threads),
		}
		suite.replaceConfig(doc)
		ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			var domain libvirtxml.Domain
			if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) || !asrt.NotNil(domain.CPU) {
				return
			}

			asrt.Equal(&libvirtxml.DomainCPUTopology{
				Sockets: int(topology.Sockets),
				Cores:   int(topology.Cores),
				Threads: int(topology.Threads),
			}, domain.CPU.Topology)
			asrt.Equal(uint(doc.CPUConfig.CPUCount), domain.VCPU.Value)
			asrt.Nil(domain.CPUTune)
			asrt.NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
		})
	}

	doc.CPUConfig.CPULimit = "2000m"
	doc.CPUConfig.TopologyConfig.PinningConfig = hypervisorcfg.VirtualMachineCPUPinning{
		VCPUsConfig:    []hypervisorcfg.VirtualMachineVCPUPin{{PinVCPU: 0, PinCPUs: "4"}},
		EmulatorConfig: "5",
	}
	suite.replaceConfig(doc)
	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		var domain libvirtxml.Domain
		if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) || !asrt.NotNil(domain.CPU) || !asrt.NotNil(domain.CPUTune) {
			return
		}

		asrt.Equal(&libvirtxml.DomainCPUTopology{Sockets: 1, Cores: 2, Threads: 2}, domain.CPU.Topology)
		asrt.Equal([]libvirtxml.DomainCPUTuneVCPUPin{{VCPU: 0, CPUSet: "4"}}, domain.CPUTune.VCPUPin)
		asrt.Equal(&libvirtxml.DomainCPUTuneEmulatorPin{CPUSet: "5"}, domain.CPUTune.EmulatorPin)
		asrt.NotNil(domain.CPUTune.GlobalQuota)
		asrt.NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
	})

	doc.CPUConfig.CPUCount = 3
	doc.CPUConfig.CPULimit = ""
	doc.CPUConfig.TopologyConfig = hypervisorcfg.VirtualMachineCPUTopology{}
	suite.replaceConfig(doc)
	suite.assertDomain(doc.Name(), "default")
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidCPUTopology() {
	for i, topology := range []hypervisor.VirtualMachineCPUTopologySpec{
		{},
		{Sockets: 1, Cores: 3},
		{Sockets: 1, Threads: 3},
		{Cores: 3, Threads: 1},
		{Sockets: 1, Cores: 2, Threads: 1},
		{Sockets: 1 << 31, Cores: 1 << 31, Threads: 4},
		// The product is 2^64 + 4, which must not wrap and match the four-vCPU count.
		{Sockets: 111620, Cores: 429509837, Threads: 384773},
	} {
		name := fmt.Sprintf("invalid-topology-%d", i)
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 4, Topology: &topology},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
			PowerState: "running",
		}
		suite.Create(spec)
		suite.assertConversionError(name, "CPU topology")
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, name)
		suite.Destroy(spec)
	}
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidPlacement() {
	for _, test := range []struct {
		name   string
		cpu    hypervisor.VirtualMachineCPUSpec
		numa   *hypervisor.VirtualMachineMemoryNUMASpec
		reason string
	}{
		{
			name:   "vcpu past count",
			cpu:    hypervisor.VirtualMachineCPUSpec{Count: 3, Pins: []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 3, CPUs: "0"}}},
			reason: "pinned vCPU 3 must be less than CPU count 3",
		},
		{
			name:   "duplicate vcpu",
			cpu:    hypervisor.VirtualMachineCPUSpec{Count: 3, Pins: []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 1, CPUs: "0"}, {VCPU: 1, CPUs: "1"}}},
			reason: "vCPU 1 is pinned more than once",
		},
		{
			name:   "empty vcpu pin",
			cpu:    hypervisor.VirtualMachineCPUSpec{Count: 3, Pins: []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 1}}},
			reason: "vCPU 1 pin must name at least one host ID",
		},
		{
			name:   "vcpu pin past the host bound",
			cpu:    hypervisor.VirtualMachineCPUSpec{Count: 3, Pins: []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 1, CPUs: "0-1000000000"}}},
			reason: `vCPU 1 pin "0-1000000000": 1000000000 is out of range`,
		},
		{
			name:   "emulator pin exclusion grammar",
			cpu:    hypervisor.VirtualMachineCPUSpec{Count: 3, EmulatorPin: "0-3,^1"},
			reason: `emulator pin "0-3,^1"`,
		},
		{
			name:   "numa mode",
			cpu:    hypervisor.VirtualMachineCPUSpec{Count: 3},
			numa:   &hypervisor.VirtualMachineMemoryNUMASpec{Mode: "bind", Nodes: "0"},
			reason: `unsupported NUMA mode "bind"`,
		},
		{
			name:   "numa nodes past the mask",
			cpu:    hypervisor.VirtualMachineCPUSpec{Count: 3},
			numa:   &hypervisor.VirtualMachineMemoryNUMASpec{Mode: "strict", Nodes: "16384"},
			reason: `NUMA nodes "16384": 16384 is out of range`,
		},
		{
			name:   "numa nodes empty",
			cpu:    hypervisor.VirtualMachineCPUSpec{Count: 3},
			numa:   &hypervisor.VirtualMachineMemoryNUMASpec{Mode: "strict"},
			reason: "NUMA nodes must name at least one host ID",
		},
	} {
		suite.Run(test.name, func() {
			suite.logs.TakeAll()

			spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "placement-invalid")
			*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
				CPU:        test.cpu,
				Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024, NUMA: test.numa},
				PowerState: "running",
				Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
				Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
			}

			suite.Create(spec)
			suite.assertConversionError("placement-invalid", test.reason)
			ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "placement-invalid")
			suite.Destroy(spec)
		})
	}
}

// A spec written by another producer need not be canonical; the rendered cpuset is.
func (suite *VirtualMachineSpecSuite) TestInjectedNonCanonicalPlacement() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU: hypervisor.VirtualMachineCPUSpec{
			Count:       3,
			Pins:        []hypervisor.VirtualMachineVCPUPinSpec{{VCPU: 0, CPUs: "8"}, {VCPU: 1, CPUs: "10,9,+9"}},
			EmulatorPin: "1,0",
		},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30, NUMA: &hypervisor.VirtualMachineMemoryNUMASpec{Mode: "strict", Nodes: "1,1"}},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)
	suite.assertDomain("guest-one", "cpu-pinning")
}

func (suite *VirtualMachineSpecSuite) TestRejectsCPULimitOutsideSchemaRange() {
	// Below the schema's 1000 microsecond minimum quota, and above its maximum.
	for _, limit := range []uint64{1, 9, 17592186044415*1000/100000 + 1} {
		suite.logs.TakeAll()

		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "capped-invalid")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1, Limit: limit},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		}

		suite.Create(spec)
		suite.assertConversionError("capped-invalid", "CPU limit must be between 10 and")
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "capped-invalid")
		suite.Destroy(spec)
	}
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidVideo() {
	for _, video := range []hypervisor.VirtualMachineVideoSpec{
		{Model: "qxl"},
		{Model: "unknown"},
		{Model: "virtio", VRAMMiB: 32},
		{Model: "cirrus", VRAMMiB: 32},
		{Model: "vga", VRAMMiB: 1024},
		{Model: "bochs", VRAMMiB: 2},
	} {
		suite.logs.TakeAll()

		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "video-invalid")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
			PowerState: "stopped",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Console:    hypervisor.VirtualMachineConsoleSpec{VNC: true},
			Video:      video,
		}
		suite.Create(spec)

		reason := "video.vramMiB"
		if video.Model == "qxl" || video.Model == "unknown" {
			reason = "unsupported video.model"
		}

		suite.assertConversionError("video-invalid", reason)
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "video-invalid")
		suite.Destroy(spec)
	}
}

func (suite *VirtualMachineSpecSuite) TestVideoLifecycle() {
	doc := newVirtualMachine("video")
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	for _, video := range []hypervisorcfg.VirtualMachineVideo{
		{ModelConfig: "bochs", VRAMMiBConfig: 32},
		{ModelConfig: "vga", VRAMMiBConfig: 64},
		{},
	} {
		doc.VideoConfig = video
		suite.replaceConfig(doc)
		ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachineVideoSpec{Model: video.ModelConfig, VRAMMiB: video.VRAMMiBConfig}, res.TypedSpec().Video)
		})
	}
}

func (suite *VirtualMachineSpecSuite) replaceConfig(docs ...configcfg.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)
	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)

	res := config.NewMachineConfig(cfg)
	res.Metadata().SetVersion(old.Metadata().Version())
	suite.Update(res)
}

func (suite *VirtualMachineSpecSuite) assertWaitingForLink(reason string) {
	suite.T().Helper()
	suite.Require().Eventually(func() bool {
		return suite.logs.Filter(func(entry observer.LoggedEntry) bool {
			err, ok := entry.ContextMap()["error"].(string)

			return ok && entry.Level < zap.ErrorLevel && strings.Contains(err, reason)
		}).Len() > 0
	}, 30*time.Second, time.Millisecond)

	suite.Assert().Empty(suite.logs.FilterLevelExact(zap.ErrorLevel).All())
}

func (suite *VirtualMachineSpecSuite) createLink(id, alias string) {
	suite.createLinkOfType(id, alias, nethelpers.LinkEther)
}

func (suite *VirtualMachineSpecSuite) createLinkOfType(id, alias string, linkType nethelpers.LinkType) {
	link := network.NewLinkStatus(network.NamespaceName, id)
	link.TypedSpec().Type = linkType
	link.TypedSpec().Alias = alias
	suite.Create(link)
}

func (suite *VirtualMachineSpecSuite) TestInterfacesAttachToResolvedLinks() {
	suite.createLink("eth0", "")
	suite.createLink("macvlan0", "uplink")

	doc := newVirtualMachine("interfaces")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "eth0"},
		{InterfaceName: "net1", InterfaceLink: "uplink"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.assertDomain(doc.Name(), "interfaces")
}

// The fixture pins MACs derived from the names; this checks they also differ per interface and per
// virtual machine.
func (suite *VirtualMachineSpecSuite) TestInterfaceMACsAreDistinct() {
	suite.createLink("eth0", "")

	macs := map[string]struct{}{}

	for _, name := range []string{"mac-one", "mac-two"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{
				{Name: "net0", Link: "eth0"},
				{Name: "net1", Link: "eth0"},
			},
		}
		suite.Create(spec)

		ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			found := regexp.MustCompile(`<mac address="([^"]+)">`).FindAllStringSubmatch(res.TypedSpec().DomainXML, -1)
			asrt.Len(found, 2)

			for _, match := range found {
				asrt.Regexp(`^52:54:00:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}$`, match[1])
				macs[match[1]] = struct{}{}
			}
		})
	}

	suite.Assert().Len(macs, 4)
}

func (suite *VirtualMachineSpecSuite) TestMissingLinkBlocksRenderUntilItAppears() {
	suite.logs.TakeAll()

	doc := newVirtualMachine("missing-link")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "uplink"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.assertWaitingForLink(`virtual machine "missing-link": interface "net0": host link not found: "uplink"`)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())

	suite.createLink("macvlan0", "uplink")

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, `<source dev="macvlan0" mode="bridge">`)
	})
}

// A link that goes away after the domain was rendered costs the spec its power intent, so
// VirtualMachineController stops the guest instead of leaving it on a definition that can no
// longer be reproduced. The last good definition stays, and the intent returns with the link.
func (suite *VirtualMachineSpecSuite) TestVanishedLinkStopsVirtualMachine() {
	suite.createLink("macvlan0", "uplink")

	doc := newVirtualMachine("vanished-link")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "uplink"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", res.TypedSpec().PowerState)
	})

	link, err := safe.StateGetByID[*network.LinkStatus](suite.Ctx(), suite.State(), "macvlan0")
	suite.Require().NoError(err)
	suite.Destroy(link)

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", res.TypedSpec().PowerState)
		asrt.Contains(res.TypedSpec().DomainXML, `<source dev="macvlan0" mode="bridge">`)
	})

	suite.createLink("macvlan0", "uplink")

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", res.TypedSpec().PowerState)
	})
}

// A macvtap needs an Ethernet lower link. A link of any other type resolves but cannot carry one,
// so it is rejected here rather than at domain start.
func (suite *VirtualMachineSpecSuite) TestNonEthernetLinkIsRejected() {
	suite.logs.TakeAll()
	suite.createLinkOfType("lo", "", nethelpers.LinkLoopbck)

	doc := newVirtualMachine("non-ethernet-link")
	doc.NetworkingConfig.InterfacesConfig = []hypervisorcfg.VirtualMachineInterface{
		{InterfaceName: "net0", InterfaceLink: "lo"},
	}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.assertWaitingForLink(`virtual machine "non-ethernet-link": interface "net0": host link is not an Ethernet link: "lo"`)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
}

// A configured address is what the guest gets, so it can be reserved against on the segment the
// virtual machine lands on.
func (suite *VirtualMachineSpecSuite) TestInterfaceHardwareAddressIsHonoured() {
	suite.createLink("eth0", "")

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "mac-pinned")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Interfaces: []hypervisor.VirtualMachineInterfaceSpec{
			{Name: "net0", Link: "eth0", HardwareAddr: "52:54:00:00:31:31"},
			// Left to be derived, so both paths are covered in one domain.
			{Name: "net1", Link: "eth0"},
		},
	}
	suite.Create(spec)

	ctest.AssertResource(suite, "mac-pinned", func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		found := regexp.MustCompile(`<mac address="([^"]+)">`).FindAllStringSubmatch(res.TypedSpec().DomainXML, -1)
		asrt.Len(found, 2)

		if len(found) != 2 {
			return
		}

		asrt.Equal("52:54:00:00:31:31", found[0][1])
		asrt.Regexp(`^52:54:00:[0-9a-f]{2}:[0-9a-f]{2}:[0-9a-f]{2}$`, found[1][1])
		asrt.NotEqual("52:54:00:00:31:31", found[1][1], "the derived address must not collide with the configured one")
	})
}

// An address the guest could never answer to fails that virtual machine rather than being passed
// to libvirt, which would reject the whole domain.
func (suite *VirtualMachineSpecSuite) TestInterfaceRejectsBadHardwareAddresses() {
	suite.createLink("eth0", "")

	for _, test := range []struct {
		name string
		addr string
		want string
	}{
		{name: "malformed", addr: "not-a-mac", want: "address"},
		{name: "multicast", addr: "01:54:00:00:31:31", want: "unicast"},
		{name: "too-short", addr: "52:54:00:00:31", want: "unicast"},
	} {
		vmName := "mac-bad-" + test.name

		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{
				{Name: "net0", Link: "eth0", HardwareAddr: test.addr},
			},
		}
		suite.Create(spec)

		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, vmName)
	}
}
