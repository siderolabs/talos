// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

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

func (suite *VirtualMachineSpecSuite) TestExternalSpecRejectsConfigCollision() {
	for _, owner := range []string{"", "external"} {
		spec := hypervisor.NewVirtualMachineSpec(hypervisor.NamespaceName, "guest-one")
		*spec.TypedSpec() = hypervisor.VirtualMachineSpecSpec{
			CPU:        hypervisor.VirtualMachineCPUSpec{Count: 3},
			Memory:     hypervisor.VirtualMachineMemorySpec{Size: 4 << 30},
			PowerState: "running",
			Firmware:   hypervisor.VirtualMachineFirmwareSpec{Type: "uefi"},
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
