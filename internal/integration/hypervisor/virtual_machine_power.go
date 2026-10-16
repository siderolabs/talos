// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"fmt"
	"strings"
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

// TestPowerAPI drives a virtual machine through the power API rather than through configuration
// patches, and checks that what it writes is the machine configuration itself.
//
// A diskless BIOS guest is enough: nothing here needs the guest to boot anything, only to be a
// running QEMU process with a power button, which every rendered domain now has.
func (suite *LibvirtSuite) TestPowerAPI() {
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

	// A graceful stop asks the guest. Whether this guest obeys is the guest's business; what Talos
	// owes is the request, the configuration it wrote, and a legible status either way.
	_, err := suite.Client.HypervisorClient.Stop(nodeCtx, &machine.VirtualMachineStopRequest{Name: name})
	suite.Require().NoError(err)

	suite.assertDeclaredPower(nodeCtx, name, hypervisorhelpers.PowerStateStopped)
	suite.assertStopMode(nodeCtx, name, hypervisorhelpers.StopModeGraceful.String())

	if suite.waitForGracefulStop(nodeCtx, node, name) {
		suite.T().Log("the guest powered itself off after the power button was pressed")
	} else {
		suite.T().Log("the guest did not power itself off; a forced stop is the way out of that")
	}

	// Forced is the escape from a graceful stop a guest refuses, and the only stop a guest cannot
	// refuse, so after it the domain is gone whichever way the branch above went.
	_, err = suite.Client.HypervisorClient.Stop(nodeCtx, &machine.VirtualMachineStopRequest{Name: name, Force: true})
	suite.Require().NoError(err)

	suite.assertDeclaredPower(nodeCtx, name, hypervisorhelpers.PowerStateStopped)
	suite.assertStoppedDomain(nodeCtx, node, name)

	// The record of a stop is retired once there is no domain left to stop, so a stop which is over
	// leaves nothing behind for the next one to inherit.
	suite.assertStopMode(nodeCtx, name, "")

	// Rebooting something which is not running has nothing to ask.
	_, err = suite.Client.HypervisorClient.Reboot(nodeCtx, &machine.VirtualMachineRebootRequest{Name: name})
	suite.Require().Error(err, "a stopped virtual machine cannot be rebooted")

	_, err = suite.Client.HypervisorClient.Start(nodeCtx, &machine.VirtualMachineStartRequest{Name: name})
	suite.Require().NoError(err)

	suite.assertDeclaredPower(nodeCtx, name, hypervisorhelpers.PowerStateRunning)
	suite.assertRunningTransientDomainWithDevices(node, name, 1, 0, 0)

	// A graceful reboot resets the guest inside the process it is already running in.
	before := suite.qemuPID(nodeCtx, name)
	suite.Require().NotZero(before)

	_, err = suite.Client.HypervisorClient.Reboot(nodeCtx, &machine.VirtualMachineRebootRequest{Name: name})
	suite.Require().NoError(err)

	suite.Require().Never(func() bool {
		return suite.qemuPID(nodeCtx, name) != before
	}, 15*time.Second, time.Second, "a graceful reboot must keep the domain it was started from")

	// A forced one destroys the domain, and the definition which is live brings it back.
	_, err = suite.Client.HypervisorClient.Reboot(nodeCtx, &machine.VirtualMachineRebootRequest{Name: name, Force: true})
	suite.Require().NoError(err)

	suite.Require().Eventually(func() bool {
		pid := suite.qemuPID(nodeCtx, name)

		return pid != 0 && pid != before
	}, 2*time.Minute, time.Second, "a forced reboot must have the domain defined again")

	suite.assertRunningTransientDomainWithDevices(node, name, 1, 0, 0)
	suite.assertDeclaredPower(nodeCtx, name, hypervisorhelpers.PowerStateRunning)
}

// assertDeclaredPower reads back what the power API wrote into the machine configuration, which is
// the thing that makes a stop outlive a reboot of the host.
func (suite *LibvirtSuite) assertDeclaredPower(ctx context.Context, name string, powerState hypervisorhelpers.PowerState) {
	suite.T().Helper()

	suite.Require().Eventually(func() bool {
		provider, err := suite.ReadConfigFromNode(ctx)
		if err != nil {
			return false
		}

		for _, vm := range provider.VirtualMachineConfigs() {
			if vm.Name() == name {
				return vm.PowerState() == powerState
			}
		}

		return false
	}, time.Minute, time.Second, "machine configuration does not declare %s for %q", powerState, name)
}

// assertStopMode reads back how the next stop was asked to be carried out. Unlike the power state
// this lives outside the machine configuration: it is a parameter of a transition, not a state, and
// an empty mode is a stop which is over and whose record has been retired.
func (suite *LibvirtSuite) assertStopMode(ctx context.Context, name, mode string) {
	suite.T().Helper()

	if mode == "" {
		rtestutils.AssertNoResource[*hypervisor.VirtualMachineStopMode](ctx, suite.T(), suite.Client.COSI, name)

		return
	}

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(res *hypervisor.VirtualMachineStopMode, asrt *assert.Assertions) {
			asrt.Equal(mode, res.TypedSpec().Mode)
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
			asrt.Equal(hypervisor.VirtualMachineStagePending, status.TypedSpec().Stage)
			asrt.Equal(hypervisor.GracefulStopPendingError, status.TypedSpec().Error)
		},
	)

	return false
}

// qemuPID reports the process the named virtual machine is running in, or zero when it is not.
func (suite *LibvirtSuite) qemuPID(ctx context.Context, name string) int32 {
	suite.T().Helper()

	response, err := suite.Client.Processes(ctx)
	if err != nil {
		if base.IgnoreGRPCUnavailable(err) == nil {
			return 0
		}

		suite.Require().NoError(err)
	}

	for _, message := range response.Messages {
		for _, process := range message.Processes {
			if strings.Contains(process.Executable, "/qemu-system-") && strings.Contains(process.Args, fmt.Sprintf("guest=%s,", name)) {
				return process.Pid
			}
		}
	}

	return 0
}
