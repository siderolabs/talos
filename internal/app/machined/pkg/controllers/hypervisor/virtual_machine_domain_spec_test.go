// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

//nolint:gocyclo // Verify both architecture-specific defaults across console lifecycle transitions.
func (suite *VirtualMachineSpecSuite) TestVNCDevices() {
	for _, firmware := range []string{"bios", "uefi"} {
		for _, serial := range []bool{false, true} {
			name := fmt.Sprintf("vnc-%s-serial-%t", firmware, serial)
			spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
			*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
				CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
				Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
				PowerState: "stopped",
				Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: firmware},
				Console:    hypervisor.VirtualMachineConsoleSpec{Serial: serial},
			}
			suite.Create(spec)

			// Exercise enabling, disabling and re-enabling without leaving stale devices.
			for _, vnc := range []bool{false, true, false, true} {
				ctest.UpdateWithConflicts(suite, spec, func(current *hypervisor.VirtualMachineSpec) error {
					current.TypedSpec().Console.VNC = vnc

					return nil
				})
				ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
					var domain libvirtxml.Domain
					if !asrt.NoError(domain.Unmarshal(res.TypedSpec().DomainXML)) || !asrt.NotNil(domain.Devices) {
						return
					}

					if serial {
						if asrt.Len(domain.Devices.Serials, 1) {
							asrt.Equal(&libvirtxml.DomainChardevSource{
								Pty: &libvirtxml.DomainChardevSourcePty{},
							}, domain.Devices.Serials[0].Source)
						}
					} else {
						asrt.Empty(domain.Devices.Serials)
					}

					if vnc {
						if asrt.Len(domain.Devices.Graphics, 1) {
							// Exact equality excludes TCP, websocket and prescribed socket paths.
							asrt.Equal(&libvirtxml.DomainGraphicVNC{
								Listeners: []libvirtxml.DomainGraphicListener{{Socket: &libvirtxml.DomainGraphicListenerSocket{}}},
							}, domain.Devices.Graphics[0].VNC)
						}
					} else {
						asrt.Empty(domain.Devices.Graphics)
					}

					if vnc && runtime.GOARCH == "amd64" {
						if asrt.Len(domain.Devices.Videos, 1) {
							asrt.Equal("vga", domain.Devices.Videos[0].Model.Type)
						}

						if asrt.Len(domain.Devices.Controllers, 1) {
							asrt.Equal("usb", domain.Devices.Controllers[0].Type)
							asrt.Equal("qemu-xhci", domain.Devices.Controllers[0].Model)
						}

						if asrt.Len(domain.Devices.Inputs, 2) {
							asrt.Equal("tablet", domain.Devices.Inputs[0].Type)
							asrt.Equal("usb", domain.Devices.Inputs[0].Bus)
							asrt.Equal("keyboard", domain.Devices.Inputs[1].Type)
							asrt.Equal("usb", domain.Devices.Inputs[1].Bus)
						}
					} else {
						asrt.Empty(domain.Devices.Videos)
						asrt.Empty(domain.Devices.Controllers)
						asrt.Empty(domain.Devices.Inputs)
					}
				})

				rendered, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), name)
				suite.Require().NoError(err)
				suite.Require().NoError(validateDomainXML([]byte(rendered.TypedSpec().DomainXML)))
			}
		}
	}
}

// Both production controllers remain registered in these tests. No MachineConfig
// is needed to create, update, validate, or remove externally authored specs.
func (suite *VirtualMachineSpecSuite) TestExternalSpecLifecycle() {
	suite.externalSpecLifecycle("external")
}

func (suite *VirtualMachineSpecSuite) TestUnownedSpecLifecycle() {
	suite.externalSpecLifecycle("")
}

func (suite *VirtualMachineSpecSuite) externalSpecLifecycle(owner string) {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "balloon")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU: hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory: hypervisor.VirtualMachineMemorySpec{
			Size:       4 << 30,
			Ballooning: hypervisor.VirtualMachineMemoryBallooningSpec{Enabled: true},
		},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), spec, state.WithCreateOwner(owner)))
	suite.assertDomain("balloon", "balloon-enabled")
	ctest.AssertNoResource[*config.MachineConfig](suite, config.ActiveID)

	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), "balloon")
	suite.Require().NoError(err)

	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 7},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Require().NoError(suite.State().Update(suite.Ctx(), spec, state.WithUpdateOwner(owner)))
	suite.assertDomain("balloon", "balloon-omitted")

	// A config reconcile and subsequent cleanup must leave external input alone.
	cfg, err := container.New(newVirtualMachine("configured"))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, "configured", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	suite.Destroy(config.NewMachineConfig(cfg))
	ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, "configured")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "configured")
	ctest.AssertResource(suite, "balloon", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal(owner, res.Metadata().Owner())
		asrt.Equal(spec.TypedSpec(), res.TypedSpec())
	})
	suite.assertDomain("balloon", "balloon-omitted")

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), spec.Metadata(), state.WithDestroyOwner(owner)))
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "balloon")
}

func (suite *VirtualMachineSpecSuite) TestBIOSDomainUsesDefaultFirmware() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "bios-default")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 128 << 20},
		PowerState: "stopped",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "bios"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)
	ctest.AssertResource(suite, spec.Metadata().ID(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, "<os>")
		asrt.NotContains(res.TypedSpec().DomainXML, `firmware="bios"`)
		asrt.Equal("stopped", res.TypedSpec().PowerState)
	})
}

// Every domain gets ACPI, not only the UEFI ones which cannot boot without it: without it a
// guest has no power button, so it could only ever be stopped by being destroyed.
func (suite *VirtualMachineSpecSuite) TestEveryDomainEnablesACPI() {
	for _, firmware := range []string{"bios", "uefi"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, firmware+"-acpi")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
			PowerState: "stopped",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: firmware},
		}
		suite.Create(spec)
		ctest.AssertResource(suite, spec.Metadata().ID(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Contains(res.TypedSpec().DomainXML, "<acpi></acpi>")
		})
	}
}

// The stop mode asked for through the API reaches the controller which takes the domain down, and
// a virtual machine nothing asked anything of is stopped the way every stop used to be.
func (suite *VirtualMachineSpecSuite) TestStopModeReachesTheDomainSpec() {
	for _, stopMode := range []string{"", hypervisorhelpers.StopModeGraceful.String(), hypervisorhelpers.StopModeForced.String()} {
		name := "stop-mode-" + stopMode

		if stopMode != "" {
			marker := hypervisor.NewVirtualMachineStopMode(hypervisor.NamespaceName, name)
			marker.TypedSpec().Mode = stopMode
			suite.Create(marker)
		}

		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		}
		suite.Create(spec)
		ctest.AssertResource(suite, name, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Equal(stopMode, res.TypedSpec().StopMode)
		})
	}
}

func (suite *VirtualMachineSpecSuite) TestDomainSpecCleanupWaitsForFinalizer() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "held-domain")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 512 << 20},
		PowerState: "stopped",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)

	domain := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, spec.Metadata().ID())
	ctest.AssertResource(suite, domain.Metadata().ID(), func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	suite.AddFinalizer(domain.Metadata(), "test.libvirt-cleanup")
	suite.Destroy(spec)

	ctest.AssertResource(suite, domain.Metadata().ID(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, res.Metadata().Phase())
		asrt.True(res.Metadata().Finalizers().Has("test.libvirt-cleanup"))
	})
	suite.RemoveFinalizer(domain.Metadata(), "test.libvirt-cleanup")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, domain.Metadata().ID())
}

func (suite *VirtualMachineSpecSuite) TestExternalSpecRejectsConfigCollision() {
	for _, owner := range []string{"", "external"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		}
		suite.Require().NoError(suite.State().Create(suite.Ctx(), spec, state.WithCreateOwner(owner)))
		suite.assertDomain("guest-one", "default")
		suite.logs.TakeAll()

		doc := newVirtualMachine("guest-one")
		doc.CPUConfig.CPUCount = 7
		cfg, err := container.New(doc)
		suite.Require().NoError(err)
		suite.Create(config.NewMachineConfig(cfg))
		suite.Require().Eventually(func() bool {
			return suite.logs.Filter(func(entry observer.LoggedEntry) bool {
				message, ok := entry.ContextMap()["error"].(string)

				return ok && strings.Contains(message, `failed to write virtual machine spec "guest-one"`)
			}).Len() > 0
		}, 3*time.Second, time.Millisecond)
		ctest.AssertResource(suite, "guest-one", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.Equal(owner, res.Metadata().Owner())
			asrt.Equal(spec.TypedSpec(), res.TypedSpec())
		})
		suite.assertDomain("guest-one", "default")

		// Clear the conflict and force a successful config pass before checking cleanup.
		suite.replaceConfig(newVirtualMachine("barrier"))
		ctest.AssertResource(suite, "barrier", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
		suite.Destroy(config.NewMachineConfig(cfg))
		ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, "barrier")
		suite.assertDomain("guest-one", "default")
		suite.Require().NoError(suite.State().Destroy(suite.Ctx(), spec.Metadata(), state.WithDestroyOwner(owner)))
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "guest-one")
	}
}

func (suite *VirtualMachineSpecSuite) TestPersistentCollisionDoesNotBlockProjectionOrCleanup() {
	cfg, err := container.New(newVirtualMachine("removed"), newVirtualMachine("retained"))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))
	ctest.AssertResource(suite, "removed", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})
	ctest.AssertResource(suite, "retained", func(_ *hypervisor.VirtualMachineDomainSpec, _ *assert.Assertions) {})

	external := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
	*external.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Require().NoError(suite.State().Create(suite.Ctx(), external, state.WithCreateOwner("external")))
	suite.assertDomain("guest-one", "default")

	// A phase conflict exercises failed modification tracking for an owned spec.
	retained, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), "retained")
	suite.Require().NoError(err)
	_, err = suite.State().Teardown(suite.Ctx(), retained.Metadata(), state.WithTeardownOwner(retained.Metadata().Owner()))
	suite.Require().NoError(err)

	collision := newVirtualMachine("guest-one")
	collision.CPUConfig.CPUCount = 7
	retainedDoc := newVirtualMachine("retained")
	retainedDoc.CPUConfig.CPUCount = 7
	suite.replaceConfig(collision, retainedDoc, newVirtualMachine("balloon"))
	suite.assertDomain("balloon", "balloon-disabled")
	ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, "removed")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "removed")
	ctest.AssertResource(suite, "retained", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal(retained.Metadata().Owner(), res.Metadata().Owner())
		asrt.Equal(retained.TypedSpec(), res.TypedSpec())
	})
	ctest.AssertResource(suite, "guest-one", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
		asrt.Equal("external", res.Metadata().Owner())
		asrt.Equal(external.TypedSpec(), res.TypedSpec())
	})
	suite.assertDomain("guest-one", "default")
}

func (suite *VirtualMachineSpecSuite) TestEqualConfigCollisionPreservesExternalOwnership() {
	for _, owner := range []string{"", "external"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		}
		suite.Require().NoError(suite.State().Create(suite.Ctx(), spec, state.WithCreateOwner(owner)))
		suite.assertDomain("guest-one", "default")

		cfg, err := container.New(newVirtualMachine("guest-one"), newVirtualMachine("barrier"))
		suite.Require().NoError(err)
		suite.logs.TakeAll()
		suite.Create(config.NewMachineConfig(cfg))
		ctest.AssertResource(suite, "barrier", func(_ *hypervisor.VirtualMachineSpec, _ *assert.Assertions) {})
		suite.Destroy(config.NewMachineConfig(cfg))
		ctest.AssertNoResource[*hypervisor.VirtualMachineSpec](suite, "barrier")
		ctest.AssertResource(suite, "guest-one", func(res *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.Equal(owner, res.Metadata().Owner())
			asrt.Equal(spec.TypedSpec(), res.TypedSpec())
			asrt.Equal(spec.Metadata().Version(), res.Metadata().Version())
		})
		suite.Empty(suite.logs.FilterLevelExact(zap.ErrorLevel).All())
		suite.assertDomain("guest-one", "default")
		suite.Require().NoError(suite.State().Destroy(suite.Ctx(), spec.Metadata(), state.WithDestroyOwner(owner)))
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "guest-one")
	}
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidNames() {
	for _, name := range []string{"", "bad\nname", "bad\x00name", "bad\x01name", "bad\ufffename", "bad\uffffname", "bad\xffname", "bad\xed\xa0\x80name"} {
		suite.Run(fmt.Sprintf("%q", name), func() {
			suite.logs.TakeAll()

			spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, name)
			*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
				CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
				Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
				PowerState: "running",
				Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
				Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
			}

			suite.Create(spec)
			defer suite.Destroy(spec)

			suite.assertConversionError(name, "")
			ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, name)
		})
	}
}

func (suite *VirtualMachineSpecSuite) TestInjectedEscapedName() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest/&<\u96ea>\t\r🙂")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)
	suite.assertDomain(spec.Metadata().ID(), "escaped-name")
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidSpecs() {
	for _, invalid := range []hypervisor.VirtualMachineSpecSpec{
		// Missing power and firmware are independently invalid even with valid CPU/memory.
		{
			CPU:      hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:   hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Firmware: hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:    hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		},
		{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			PowerState: "running",
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 0},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1024},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 65536},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1024},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 4294967295},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1024},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 0},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1023},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1025},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1 << 63},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1<<64 - 1},
		},
		// Interface names become libvirt user aliases and must be unique; links are required.
		{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{{Name: "", Link: "eth0"}},
		},
		{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{{Name: "net.0", Link: "eth0"}},
		},
		{
			CPU:    hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory: hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{
				{Name: "net0", Link: "eth0"},
				{Name: "net0", Link: "eth1"},
			},
		},
		{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1024},
			Interfaces: []hypervisor.VirtualMachineInterfaceSpec{{Name: "net0"}},
		},
	} {
		suite.logs.TakeAll()

		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "invalid")

		*spec.TypedSpec() = invalid
		if invalid.PowerState == "" && invalid.Firmware.Type == "" {
			// CPU/memory cases must reach their own validation, not fail on missing intent.
			spec.TypedSpec().PowerState = "running"
			spec.TypedSpec().Firmware.Type = "uefi"
		}

		suite.Create(spec)

		reason := ""
		if invalid.Firmware.Type != "" && invalid.PowerState == "" {
			reason = `unsupported power state ""`
		} else if invalid.PowerState != "" && invalid.Firmware.Type == "" {
			reason = `unsupported firmware type ""`
		}

		suite.assertConversionError("invalid", reason)
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "invalid")
		suite.Destroy(spec)

		// A successful render resets restart backoff between invalid cases.
		barrier := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "balloon")
		*barrier.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
			Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		}
		suite.Create(barrier)
		suite.assertDomain("balloon", "balloon-disabled")
		suite.Destroy(barrier)
		ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "balloon")
	}
}

func (suite *VirtualMachineSpecSuite) TestInvalidSpecDoesNotBlockOtherDomains() {
	stale := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "balloon")
	*stale.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(stale)
	suite.assertDomain("balloon", "balloon-disabled")

	invalid := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "aaa-invalid")
	suite.Create(invalid)
	suite.assertConversionError("aaa-invalid", "")
	suite.Destroy(stale)

	healthy := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
	*healthy.TypedSpec() = *stale.TypedSpec()
	suite.Create(healthy)
	suite.assertDomain("guest-one", "default")
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "balloon")
}

func (suite *VirtualMachineSpecSuite) TestInjectedInvalidUpdateRecovers() {
	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "balloon")
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
	}
	suite.Create(spec)
	suite.assertDomain("balloon", "balloon-disabled")
	suite.logs.TakeAll()

	spec, err := safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), "balloon")
	suite.Require().NoError(err)

	spec.TypedSpec().Memory.Size++
	suite.Update(spec)
	suite.assertConversionError("balloon", "memory size must be a multiple of 1024 bytes")
	suite.assertDomain("balloon", "balloon-disabled")

	spec, err = safe.StateGetByID[*hypervisor.VirtualMachineSpec](suite.Ctx(), suite.State(), "balloon")
	suite.Require().NoError(err)

	spec.TypedSpec().Memory.Size--
	spec.TypedSpec().Memory.Ballooning.Enabled = true
	suite.Update(spec)
	suite.assertDomain("balloon", "balloon-enabled")
	suite.Destroy(spec)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, "balloon")
}

// Register an alternate COSI producer, rather than merely asserting an output
// declaration. The runtime must allow it alongside the machine-config projector.
func (suite *VirtualMachineSpecSuite) TestAllowsSharedSpecProducer() {
	suite.Require().NoError(suite.Runtime().RegisterController(&externalSpecProducer{}))
}

type externalSpecProducer struct {
	hypervisorctrl.VirtualMachineSpecController
}

func (*externalSpecProducer) Name() string {
	return "external.VirtualMachineSpecController"
}

func (*externalSpecProducer) Inputs() []controller.Input {
	return nil
}

func (*externalSpecProducer) Run(ctx context.Context, _ controller.Runtime, _ *zap.Logger) error {
	<-ctx.Done()

	return nil
}

// contentLibraryPlaceholder stands in for the library's mount point, which is a temporary
// directory and therefore differs between runs, so that the fixture can stay exact.
const contentLibraryPlaceholder = "/content-library"

func (suite *VirtualMachineSpecSuite) TestRendersCDROMFromContentLibrary() {
	path := suite.T().TempDir()
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte("iso"), 0o600))

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	*library.TypedSpec() = hypervisor.ContentLibraryStatusSpec{VolumeID: "u-images", Path: path, Ready: true}
	suite.Create(library)

	doc := newVirtualMachine("booted")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName:      "install",
			DiskType:      hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskBootOrder: 1,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "images",
					ImageFile:    "talos.iso",
				},
			},
		},
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	want, err := os.ReadFile(filepath.Join("testdata", "virtualmachinespec", "cdrom.xml"))
	suite.Require().NoError(err)

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal(string(want), strings.ReplaceAll(res.TypedSpec().DomainXML, path, contentLibraryPlaceholder)+"\n")
	})

	res, err := safe.StateGetByID[*hypervisor.VirtualMachineDomainSpec](suite.Ctx(), suite.State(), doc.Name())
	suite.Require().NoError(err)
	suite.Require().NoError(validateDomainXML([]byte(res.TypedSpec().DomainXML)))
}

// Two cdroms on the same driver must not both claim sda.
func (suite *VirtualMachineSpecSuite) TestAllocatesDistinctTargetDevices() {
	path := suite.T().TempDir()
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte("iso"), 0o600))

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "two-images")
	*library.TypedSpec() = hypervisor.ContentLibraryStatusSpec{VolumeID: "u-two-images", Path: path, Ready: true}
	suite.Create(library)

	cdrom := func(name string, bus hypervisorhelpers.VirtualMachineDiskBus) hypervisorcfg.VirtualMachineDisk {
		return hypervisorcfg.VirtualMachineDisk{
			DiskName: name,
			DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskBus:  bus,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "two-images",
					ImageFile:    "talos.iso",
				},
			},
		}
	}

	doc := newVirtualMachine("two-cdroms")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		cdrom("install", hypervisorhelpers.VirtualMachineDiskBusSATA),
		cdrom("rescue", hypervisorhelpers.VirtualMachineDiskBusSCSI),
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	ctest.AssertResource(suite, doc.Name(), func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, `<target dev="sda" bus="sata"></target>`)
		asrt.Contains(res.TypedSpec().DomainXML, `<target dev="sdb" bus="scsi"></target>`)
	})
}

// libvirt has no nvme disk bus, so the domain is refused rather than rendered unusably.
func (suite *VirtualMachineSpecSuite) TestRejectsNVMeBus() {
	path := suite.T().TempDir()
	suite.Require().NoError(os.WriteFile(filepath.Join(path, "talos.iso"), []byte("iso"), 0o600))

	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "nvme-images")
	*library.TypedSpec() = hypervisor.ContentLibraryStatusSpec{VolumeID: "u-nvme-images", Path: path, Ready: true}
	suite.Create(library)

	doc := newVirtualMachine("nvme-cdrom")
	doc.DisksConfig = []hypervisorcfg.VirtualMachineDisk{
		{
			DiskName: "install",
			DiskType: hypervisorhelpers.VirtualMachineDiskTypeCDROM,
			DiskBus:  hypervisorhelpers.VirtualMachineDiskBusNVMe,
			ProvisionConfig: hypervisorcfg.VirtualMachineDiskProvision{
				FromImageConfig: &hypervisorcfg.VirtualMachineDiskFromImage{
					ImageLibrary: "nvme-images",
					ImageFile:    "talos.iso",
				},
			},
		},
	}

	cfg, err := container.New(doc)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	suite.assertConversionError(doc.Name(), `disk "install": unsupported bus "nvme"`)
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, doc.Name())
}

// VirtualMachineStaleDiskSuite runs the domain renderer without VirtualMachineDiskController, so
// that the disk statuses it reads are the ones the test writes.
type VirtualMachineStaleDiskSuite struct {
	ctest.DefaultSuite
}

func TestVirtualMachineStaleDiskSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VirtualMachineStaleDiskSuite{
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
		},
	})
}

// A disk status is keyed by what the disk is provisioned from, so a status left over from another
// image is a different resource and is simply not found. Rendering from it would attach the
// previous image, and a rendered domain is started.
func (suite *VirtualMachineStaleDiskSuite) TestWaitsOutADiskStatusForAnotherImage() {
	wanted := cdromDiskSpec("install", libraryName, "new.iso", "")

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		Disks:      []hypervisor.VirtualMachineDiskSpec{wanted},
	}
	suite.Create(spec)

	stale := cdromDiskSpec("install", libraryName, "old.iso", "")

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, stale))
	*status.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: vmName,
		Name:           "install",
		SourcePath:     filepath.Join(contentLibraryPlaceholder, "old.iso"),
		Format:         "raw",
		ReadOnly:       true,
		Ready:          true,
		Image:          hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "old.iso"},
	}
	suite.Create(status)

	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](suite, vmName)

	current := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, wanted))
	*current.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: vmName,
		Name:           "install",
		SourcePath:     filepath.Join(contentLibraryPlaceholder, "new.iso"),
		Format:         "raw",
		ReadOnly:       true,
		Ready:          true,
		Image:          hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "new.iso"},
	}
	suite.Create(current)

	ctest.AssertResource(suite, vmName, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Contains(res.TypedSpec().DomainXML, filepath.Join(contentLibraryPlaceholder, "new.iso"))
		asrt.NotContains(res.TypedSpec().DomainXML, "old.iso")
		// The definition carries what it was rendered from, so the controller which starts it can
		// hold exactly those disks.
		asrt.Equal([]string{current.Metadata().ID()}, res.TypedSpec().Disks)
	})
}

// A status held by a domain goes on existing while it tears down, but names an image the
// configuration has moved off. Rendering from it would keep a guest on a medium whose digest is
// never verified again, so the domain is stopped instead and the hold comes back with it.
func (suite *VirtualMachineStaleDiskSuite) TestStopsOnADiskStatusWhichIsTearingDown() {
	disk := cdromDiskSpec("install", libraryName, "talos.iso", "")

	// A graceful stop was asked for, which the withdrawal below is entitled to ignore.
	marker := hypervisor.NewVirtualMachineStopMode(hypervisor.NamespaceName, vmName)
	marker.TypedSpec().Mode = hypervisorhelpers.StopModeGraceful.String()
	suite.Create(marker)

	spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, vmName)
	*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
		CPU:        hypervisor.VirtualMachineCPUSpec{Count: 1},
		Memory:     hypervisor.VirtualMachineMemorySpec{Size: 1 << 30},
		PowerState: "running",
		Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
		Guest:      hypervisor.VirtualMachineGuestSpec{Agent: hypervisor.VirtualMachineAgentSpec{Enabled: true}},
		Disks:      []hypervisor.VirtualMachineDiskSpec{disk},
	}
	suite.Create(spec)

	status := hypervisor.NewVirtualMachineDiskStatus(hypervisor.NamespaceName, hypervisor.VirtualMachineDiskStatusID(vmName, disk))
	*status.TypedSpec() = hypervisor.VirtualMachineDiskStatusSpec{
		VirtualMachine: vmName,
		Name:           "install",
		SourcePath:     filepath.Join(contentLibraryPlaceholder, "talos.iso"),
		Format:         "raw",
		ReadOnly:       true,
		Ready:          true,
		Image:          hypervisor.VirtualMachineDiskFromImageSpec{Library: libraryName, File: "talos.iso"},
	}
	suite.Create(status)

	ctest.AssertResource(suite, vmName, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("running", res.TypedSpec().PowerState)
		asrt.Equal([]string{status.Metadata().ID()}, res.TypedSpec().Disks)
	})

	suite.AddFinalizer(status.Metadata(), "hypervisor.VirtualMachineController")

	ready, err := suite.State().Teardown(suite.Ctx(), status.Metadata())
	suite.Require().NoError(err)
	suite.Require().False(ready, "the hold must prevent immediate deletion")

	ctest.AssertResource(suite, vmName, func(res *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
		asrt.Equal("stopped", res.TypedSpec().PowerState)
		// A stop the controller demands to clear an unrenderable definition off the host is not
		// one the guest may refuse, whatever was asked for.
		asrt.Equal(hypervisorhelpers.StopModeForced.String(), res.TypedSpec().StopMode)
	})
}

type VirtualMachineCloudInitDomainSuite struct {
	ctest.DefaultSuite
}

func TestVirtualMachineCloudInitDomainSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &VirtualMachineCloudInitDomainSuite{
		ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.CloudInitSpecController{}))
				s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.CloudInitISOController{State: s.State()}))
				s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.VirtualMachineDomainSpecController{}))
			},
		},
	})
}

func (s *VirtualMachineCloudInitDomainSuite) TestSeedIsAttachedOnlyWhenReady() {
	vm := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest")
	vm.TypedSpec().CPU.Count = 1
	vm.TypedSpec().Memory.Size = 512 << 20
	vm.TypedSpec().PowerState = "running"
	vm.TypedSpec().Firmware.Type = "bios"
	vm.TypedSpec().CloudInit = &hypervisor.VirtualMachineCloudInitSpec{
		Library:  "images",
		MetaData: "instance-id: guest\n",
		UserData: "secret",
	}

	s.Create(vm)

	// No library: rendering must not start an unseeded guest.
	ctest.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](s, "guest")

	dir := s.T().TempDir()
	lib := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	lib.TypedSpec().Ready = true
	lib.TypedSpec().Path = dir
	lib.TypedSpec().VolumeID = "volume-a"
	s.Create(lib)

	ctest.AssertResource(s, "guest", func(domain *hypervisor.VirtualMachineDomainSpec, a *assert.Assertions) {
		a.Contains(domain.TypedSpec().DomainXML, `device="cdrom"`)
		a.Contains(domain.TypedSpec().DomainXML, `bus="sata"`)
		a.Contains(domain.TypedSpec().DomainXML, `<readonly></readonly>`)
		a.NotContains(domain.TypedSpec().DomainXML, `<boot order=`)
		a.Contains(domain.TypedSpec().DomainXML, `cloud-init-`)
		a.NotEmpty(domain.TypedSpec().CloudInit)
		a.Equal("running", domain.TypedSpec().PowerState)
	})

	// A remount invalidates the old seed even while its status is held.
	ctest.UpdateWithConflicts(s, lib, func(current *hypervisor.ContentLibraryStatus) error {
		current.TypedSpec().VolumeID = "volume-b"
		current.TypedSpec().Ready = false

		return nil
	})

	ctest.AssertResource(s, "guest", func(domain *hypervisor.VirtualMachineDomainSpec, a *assert.Assertions) {
		a.Equal("stopped", domain.TypedSpec().PowerState)
	})
}
