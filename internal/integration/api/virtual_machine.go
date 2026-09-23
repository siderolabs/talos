// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	_ "embed"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

//go:embed testdata/virtualmachinespec/created.xml
var virtualMachineCreatedXML string

//go:embed testdata/virtualmachinespec/updated.xml
var virtualMachineUpdatedXML string

// VirtualMachineSuite exercises the machine-config and resource APIs on a running node.
// It observes definitions only: no libvirt connection, guest, disk or network is needed.
// Direct desired-spec injection is not available through this harness: the public
// COSI AccessPolicy rejects all writes, including those made with administrator credentials.
type VirtualMachineSuite struct {
	base.APISuite
}

// SuiteName implements base.NamedSuite.
func (suite *VirtualMachineSuite) SuiteName() string {
	return "api.VirtualMachineSuite"
}

// TestConfigProjectionLifecycle verifies both projection layers across create, update and removal.
func (suite *VirtualMachineSuite) TestConfigProjectionLifecycle() {
	if testing.Short() {
		suite.T().Skip("skipping machine configuration changes in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(ctx, node)
	name := "vm-integration-" + uuid.NewString()

	suite.T().Logf("testing virtual machine projections %q on node %s", name, node)

	original, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	originalBytes, err := original.Bytes()
	suite.Require().NoError(err)

	for _, vm := range original.VirtualMachineConfigs() {
		suite.Require().NotEqual(name, vm.Name(), "test name collides with an existing document")
	}

	// Do not take over an existing spec from another producer, even if no document names it.
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](nodeCtx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.COSI, name)

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeUEFI
	doc.CPUConfig = hypervisorcfg.VirtualMachineCPU{
		CPUCount: 3,
	}
	doc.MemoryConfig = hypervisorcfg.VirtualMachineMemory{
		MemorySize: meta.MustByteSize("4GiB"),
		BallooningConfig: &hypervisorcfg.VirtualMachineBallooning{
			BallooningEnabled: new(true),
		},
	}

	created, err := container.New(append(slices.Clone(original.Documents()), doc)...)
	suite.Require().NoError(err)
	suite.validateVirtualMachineConfig(created, doc)

	createdBytes, err := created.Bytes()
	suite.Require().NoError(err)
	suite.validateVirtualMachineConfigBytes(createdBytes, doc)

	// Replace the whole document rather than merge it: omission must actually remove
	// the previously enabled ballooning section, not preserve it as a merge patch would.
	doc = doc.DeepCopy()
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("512MiB")
	doc.MemoryConfig.BallooningConfig = nil
	suite.Require().Equal(hypervisorhelpers.PowerStateStopped, doc.PowerStateConfig)
	suite.Require().Equal(hypervisorhelpers.VirtualMachineFirmwareTypeUEFI, doc.FirmwareConfig.FirmwareType)

	updated, err := container.New(append(slices.Clone(original.Documents()), doc)...)
	suite.Require().NoError(err)
	suite.validateVirtualMachineConfig(updated, doc)

	updatedBytes, err := updated.Bytes()
	suite.Require().NoError(err)
	suite.validateVirtualMachineConfigBytes(updatedBytes, doc)

	// Register before the first RPC: a failed apply may still have changed the node.
	// Cleanup must not reuse the test deadline, which may already have expired.
	suite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cleanupCancel()

		cleanupCtx = client.WithNode(cleanupCtx, node)

		_, applyErr := suite.Client.ApplyConfiguration(cleanupCtx, &machineapi.ApplyConfigurationRequest{
			Data: originalBytes,
			Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
		})
		if !suite.Assert().NoError(applyErr, "restore original configuration on node %s", node) {
			return
		}

		rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](cleanupCtx, suite.T(), suite.Client.COSI, name)
		rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](cleanupCtx, suite.T(), suite.Client.COSI, name)
	})

	suite.applyVirtualMachineConfig(nodeCtx, createdBytes)
	suite.assertVirtualMachineProjection(nodeCtx, name, hypervisor.VirtualMachineSpecSpec{
		PowerState: "stopped",
		Firmware: hypervisor.VirtualMachineFirmwareSpec{
			Type: "uefi",
		},
		CPU: hypervisor.VirtualMachineCPUSpec{
			Count: 3,
		},
		Memory: hypervisor.VirtualMachineMemorySpec{
			Size: 4 * 1024 * 1024 * 1024,
			Ballooning: hypervisor.VirtualMachineMemoryBallooningSpec{
				Enabled: true,
			},
		},
	}, virtualMachineCreatedXML)

	suite.applyVirtualMachineConfig(nodeCtx, updatedBytes)
	suite.assertVirtualMachineProjection(nodeCtx, name, hypervisor.VirtualMachineSpecSpec{
		PowerState: "stopped",
		Firmware: hypervisor.VirtualMachineFirmwareSpec{
			Type: "uefi",
		},
		CPU: hypervisor.VirtualMachineCPUSpec{
			Count: 1,
		},
		Memory: hypervisor.VirtualMachineMemorySpec{
			Size: 512 * 1024 * 1024,
			Ballooning: hypervisor.VirtualMachineMemoryBallooningSpec{
				Enabled: false,
			},
		},
	}, virtualMachineUpdatedXML)

	// Restoring the snapshot removes only our document, preserving all original VMs.
	suite.applyVirtualMachineConfig(nodeCtx, originalBytes)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](nodeCtx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.COSI, name)
}

func (suite *VirtualMachineSuite) validateVirtualMachineConfig(cfg *container.Container, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
	suite.T().Helper()

	// Container construction checks document identity, but not document validation.
	// The server calls ValidateAtRuntime (which includes this client-side validation)
	// before accepting a NO_REBOOT apply.
	_, err := cfg.ValidateAsClient(runtime.ModeCloud)
	suite.Require().NoError(err, "validate configuration containing virtual machine %q", doc.MetaName)
}

func (suite *VirtualMachineSuite) validateVirtualMachineConfigBytes(data []byte, doc *hypervisorcfg.VirtualMachineConfigV1Alpha1) {
	suite.T().Helper()

	// Exercise the same decoder used by ApplyConfiguration, not only the Go document.
	decoded, err := configloader.NewFromBytes(data)
	suite.Require().NoError(err)

	_, err = decoded.ValidateAsClient(runtime.ModeCloud)
	suite.Require().NoError(err)

	for _, vm := range decoded.VirtualMachineConfigs() {
		if vm.Name() == doc.MetaName {
			suite.Require().Equal(doc.PowerStateConfig, vm.PowerState())
			suite.Require().Equal(doc.FirmwareConfig.FirmwareType, vm.Firmware().Type())

			return
		}
	}

	suite.T().Fatalf("decoded configuration is missing virtual machine %q", doc.MetaName)
}

func (suite *VirtualMachineSuite) applyVirtualMachineConfig(ctx context.Context, data []byte) {
	suite.T().Helper()

	_, err := suite.Client.ApplyConfiguration(ctx, &machineapi.ApplyConfigurationRequest{
		Data: data,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})
	suite.Require().NoError(err)
}

func (suite *VirtualMachineSuite) assertVirtualMachineProjection(ctx context.Context, name string, expected hypervisor.VirtualMachineSpecSpec, fixture string) {
	suite.T().Helper()

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(spec *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.Equal(expected, *spec.TypedSpec())
		},
	)

	// Only the run-specific name varies. Compare every other byte, including the
	// explicit disabled balloon and absence of stale CPU/memory or device fields.
	expectedXML := strings.Replace(strings.TrimSuffix(fixture, "\n"), "<name>vm-integration</name>", "<name>"+name+"</name>", 1)

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.Equal(expectedXML, spec.TypedSpec().DomainXML)
		},
	)
}

func init() {
	allSuites = append(allSuites, new(VirtualMachineSuite))
}
