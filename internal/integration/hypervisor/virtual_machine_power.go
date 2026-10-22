// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// gracefulStopTimeout bounds a guest powering itself off after the power button is pressed. There
// is no deadline on the Talos side, so this one is the test's own patience.
const gracefulStopTimeout = 2 * time.Minute

// preparePowerVM creates a disposable diskless guest for host-local power tests.
func (suite *LibvirtSuite) preparePowerVM() (string, context.Context, string) {
	suite.T().Helper()

	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	name := "vm-power-" + uuid.NewString()
	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("256MiB")

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cleanupCtx := client.WithNode(ctx, node)
		suite.RemoveMachineConfigDocumentsByName(cleanupCtx, hypervisorcfg.VirtualMachineConfigKind, name)
		rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainStatus](cleanupCtx, suite.T(), suite.Client.COSI, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 0, 0)

	return node, nodeCtx, name
}

// TestPowerAPI exercises power requests and the forced escape from an uncooperative guest.
func (suite *LibvirtSuite) TestPowerAPI() {
	node, nodeCtx, name := suite.preparePowerVM()

	// A graceful stop asks the guest. Whether this guest obeys is the guest's business; what Talos
	// owes is the request, the override it wrote, and a legible status either way.
	_, err := suite.Client.HypervisorClient.Stop(nodeCtx, &machine.VirtualMachineStopRequest{Name: name})
	suite.Require().NoError(err)

	suite.assertPowerOverride(nodeCtx, name, hypervisorhelpers.PowerStateStopped.String())

	if suite.waitForGracefulStop(nodeCtx, node, name) {
		suite.T().Log("the guest powered itself off after the power button was pressed")
	} else {
		suite.T().Log("the guest did not power itself off; a forced stop is the way out of that")
	}

	// Forced is the escape from a graceful stop a guest refuses, and the only stop a guest cannot
	// refuse, so after it the domain is gone whichever way the branch above went.
	_, err = suite.Client.HypervisorClient.Stop(nodeCtx, &machine.VirtualMachineStopRequest{Name: name, Force: true})
	suite.Require().NoError(err)

	suite.assertPowerOverride(nodeCtx, name, hypervisorhelpers.PowerStateStopped.String())
	suite.assertStoppedDomain(nodeCtx, node, name)

	// The stop request of the override says how the stop was carried out.
	suite.assertStopMode(nodeCtx, name, hypervisorhelpers.StopModeForced)

	_, err = suite.Client.HypervisorClient.Start(nodeCtx, &machine.VirtualMachineStartRequest{Name: name})
	suite.Require().NoError(err)

	suite.assertPowerOverride(nodeCtx, name, "")
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 0, 0)
}

// TestAlpineGracefulPower requires guest-visible shutdown, not only an accepted RPC.
func (suite *LibvirtSuite) TestAlpineGracefulPower() {
	node, nodeCtx := suite.requireAlpineGuestNode()
	doc := suite.prepareAlpineGuestOnNode(node, nodeCtx, nil, 512<<20, "")
	name := doc.Name()
	console := suite.loginAlpineConsole(nodeCtx, name)
	nonce := uuid.NewString()
	start := console.mark()
	suite.Require().NoError(console.send("rc-service acpid start && printf '\\nPOWER_ACPID_%s_END\\n' '" + nonce + "'\n"))
	suite.Require().NoError(console.expectAfter(start, []byte("\r\nPOWER_ACPID_"+nonce+"_END\r\n")),
		"ACPI setup failed: %q", console.transcript)
	suite.detachAlpineConsole(console)

	_, err := suite.Client.HypervisorClient.Stop(nodeCtx, &machine.VirtualMachineStopRequest{Name: name})
	suite.Require().NoError(err)
	suite.assertPowerOverride(nodeCtx, name, hypervisorhelpers.PowerStateStopped.String())
	suite.assertStoppedDomain(nodeCtx, node, name)
	suite.assertStopMode(nodeCtx, name, hypervisorhelpers.StopModeGraceful)
}

// TestPowerOverrideEndsWithHostReboot verifies a stop lasts only for the boot it was asked in: the
// next boot drives the virtual machine to the power state its configuration declares.
func (suite *LibvirtSuite) TestPowerOverrideEndsWithHostReboot() {
	if testing.Short() {
		suite.T().Skip("skipping the host reboot test in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster does not support reboots")
	}

	suite.ctxCancel()
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 20*time.Minute)

	node, nodeCtx, name := suite.preparePowerVM()
	_, err := suite.Client.HypervisorClient.Stop(nodeCtx, &machine.VirtualMachineStopRequest{Name: name, Force: true})
	suite.Require().NoError(err)
	suite.assertPowerOverride(nodeCtx, name, hypervisorhelpers.PowerStateStopped.String())
	suite.assertStoppedDomain(nodeCtx, node, name)

	suite.AssertRebooted(suite.ctx, node, func(ctx context.Context) error {
		return base.IgnoreGRPCUnavailable(suite.Client.Reboot(ctx))
	}, 10*time.Minute)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})
	suite.assertPowerOverride(nodeCtx, name, "")
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 0, 0)
}

// assertPowerOverride checks the power state this VM is overridden to for the current boot, and
// that it is not overridden when powerState is empty.
func (suite *LibvirtSuite) assertPowerOverride(ctx context.Context, name, powerState string) {
	suite.T().Helper()

	if powerState == "" {
		rtestutils.AssertNoResource[*hypervisor.VirtualMachinePowerOverride](ctx, suite.T(), suite.Client.COSI, name)

		return
	}

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(override *hypervisor.VirtualMachinePowerOverride, asrt *assert.Assertions) {
			asrt.Equal(powerState, override.TypedSpec().PowerState)
		},
	)
}

// assertStopMode checks the mode of the stop request this VM is overridden with.
func (suite *LibvirtSuite) assertStopMode(ctx context.Context, name string, mode hypervisorhelpers.StopMode) {
	suite.T().Helper()

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(override *hypervisor.VirtualMachinePowerOverride, asrt *assert.Assertions) {
			asrt.Equal(mode, hypervisorhelpers.VirtualMachineStopRequestMode(override.TypedSpec().StopRequest))
		},
	)
}

// waitForGracefulStop reports whether the guest obeyed the power button, and asserts that the
// status says something useful about the wait either way.
func (suite *LibvirtSuite) waitForGracefulStop(ctx context.Context, node, name string) bool {
	suite.T().Helper()

	deadline := time.Now().Add(gracefulStopTimeout)

	for time.Now().Before(deadline) {
		if _, code := suite.RunDebugContainer(suite.ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "dominfo", name); code != 0 {
			return true
		}

		time.Sleep(time.Second)
	}

	// Still there, so the status has to name the way out of a stop with no deadline on it.
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(status *hypervisor.VirtualMachineStatus, asrt *assert.Assertions) {
			asrt.Equal(hypervisorhelpers.VirtualMachineStagePending, status.TypedSpec().Stage)
			asrt.Equal(hypervisor.GracefulStopPendingError, status.TypedSpec().Error)
		},
	)

	return false
}
