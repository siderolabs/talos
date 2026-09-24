// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
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
		},
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

func (suite *VirtualMachineProjectionSuite) TestProjectsRequiredAndOptionalIntent() {
	doc := newVirtualMachine("intent")
	doc.PowerStateConfig = hypervisorhelpers.PowerStateSuspended
	doc.FirmwareConfig.SecureBootConfig.SecureBootEnabled = new(true)
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	doc.ConsoleConfig.VNCConfig.VNCEnabled = new(true)
	doc.CPUConfig.CPULimit = "2500m"
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
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal(hypervisor.VirtualMachineSpecSpec{
			CPU: hypervisor.VirtualMachineCPUSpec{
				Count: 3,
				Limit: 2500,
			},
			Memory: hypervisor.VirtualMachineMemorySpec{
				Size: 4 << 30,
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
		asrt.Contains(res.TypedSpec().DomainXML, `<os firmware="bios">`)
	})
	ctest.AssertResource(suite, "uefi", func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, `<os firmware="efi">`)
		asrt.Contains(res.TypedSpec().DomainXML, `name="secure-boot"`)
		asrt.Contains(res.TypedSpec().DomainXML, `<serial type="pty">`)
		asrt.Contains(res.TypedSpec().DomainXML, `<graphics type="vnc"`)
	})
}

func (suite *VirtualMachineSpecSuite) TestRejectsUnresolvedDisks() {
	doc := newVirtualMachine("unresolved")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{{
		DiskName: "data", DiskPool: "pool1", DiskSize: meta.MustByteSize("20GiB"),
		ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{}},
	}}
	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal("pool1", res.TypedSpec().Disks[0].Pool)
	})
	suite.assertConversionError(doc.Name(), "unresolved disks")
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
	}, 3*time.Second, time.Millisecond)
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

func (suite *VirtualMachineSpecSuite) TestIgnoresPersistentConfigButRejectsUnresolvedDisks() {
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
				BlankConfig: &hypervisorcfg.VirtualMachineDiskBlank{},
			},
		},
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	suite.assertConversionError(doc.Name(), "unresolved disks")
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
		}

		suite.Create(spec)
		suite.assertConversionError("capped-invalid", "CPU limit must be between 10 and")
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "capped-invalid")
		suite.Destroy(spec)
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
