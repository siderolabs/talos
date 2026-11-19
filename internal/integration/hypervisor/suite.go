// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const libvirtURI = "qemu+unix:///system?socket=/run/libvirt/virtqemud-sock"

// LibvirtSuite holds the setup and helpers shared by the libvirt extension suites.
//
// It has no tests of its own: each test file is a suite embedding it, so that with
// -talos.failfast a broken area stops the run instead of every later test timing out.
type LibvirtSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SetupTest ...
func (suite *LibvirtSuite) SetupTest() {
	if !suite.ExtensionsLibvirt {
		suite.T().Skip("skipping as libvirt extension integration tests are not enabled")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	// Allow time for domain operations as well as a node reboot, and for a test to boot a guest
	// from an ISO more than once.
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 20*time.Minute)
}

// TearDownTest ...
func (suite *LibvirtSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func (suite *LibvirtSuite) assertStoppedDomain(ctx context.Context, node, name string) {
	suite.T().Helper()

	// Check backend absence independently of the managed VM's observed stopped state.
	suite.assertNoDomainWithContext(ctx, node, name)
	suite.assertVirtualMachineStatus(ctx, name, hypervisorhelpers.VirtualMachinePowerStateStopped)
}

func (suite *LibvirtSuite) assertVirtualMachineStatus(ctx context.Context, name string, powerState hypervisorhelpers.VirtualMachinePowerState) {
	suite.T().Helper()

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.State(), name,
		func(status *hypervisor.VirtualMachineStatus, asrt *assert.Assertions) {
			asrt.Equal(powerState, status.TypedSpec().PowerState)
			asrt.Equal(hypervisorhelpers.VirtualMachineStageReady, status.TypedSpec().Stage)
			asrt.Empty(status.TypedSpec().Error)
		},
	)
}

func (suite *LibvirtSuite) assertNoDomain(node, name string) {
	suite.T().Helper()

	suite.assertNoDomainWithContext(suite.ctx, node, name)
}

func (suite *LibvirtSuite) assertNoDomainWithContext(ctx context.Context, node, name string) {
	suite.T().Helper()

	suite.Require().Eventually(func() bool {
		_, listCode := suite.RunDebugContainer(ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "list", "--all")
		if listCode != 0 {
			return false
		}

		_, lookupCode := suite.RunDebugContainer(ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "dominfo", name)

		return lookupCode != 0
	}, 2*time.Minute, time.Second, "domain %q still exists", name)
}

func (suite *LibvirtSuite) assertRunningTransientDomain(node, name string, cpus uint) {
	suite.T().Helper()

	suite.assertRunningTransientDomainWithDevices(node, name, cpus, 0, 0)
}

func (suite *LibvirtSuite) assertRunningTransientDomainWithDevices(node, name string, cpus uint, disks, interfaces int) {
	suite.T().Helper()

	suite.assertVirtualMachineStatus(client.WithNode(suite.ctx, node), name, hypervisorhelpers.VirtualMachinePowerStateRunning)

	// The status contract does not include XML or persistence; verify those via libvirt.
	suite.Require().Eventually(func() bool {
		text, code := suite.RunDebugContainer(suite.ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "dumpxml", name)
		if code != 0 {
			return false
		}

		if !definedDomainMatches(text, name, cpus, disks, interfaces) {
			return false
		}

		info, infoCode := suite.RunDebugContainer(suite.ctx, node, "/usr/local/bin/virsh", "--connect", libvirtURI, "dominfo", name)

		return infoCode == 0 && regexp.MustCompile(`(?m)^Persistent:\s+no\s*$`).MatchString(info)
	}, 2*time.Minute, time.Second, "domain %q was not running transiently with %d CPUs", name, cpus)
}

func definedDomainMatches(text, name string, cpus uint, disks, interfaces int) bool {
	var domain libvirtxml.Domain

	if err := domain.Unmarshal(text); err != nil || domain.Name != name || domain.VCPU == nil || domain.VCPU.Value != cpus {
		return false
	}

	return domain.Devices != nil && len(domain.Devices.Disks) == disks && len(domain.Devices.Interfaces) == interfaces
}

func (suite *LibvirtSuite) runVirsh(node string, args ...string) string {
	command := append([]string{"/usr/local/bin/virsh", "--connect", libvirtURI}, args...)
	output, exitCode := suite.RunDebugContainer(suite.ctx, node, command...)
	suite.Require().EqualValues(0, exitCode, "virsh %s failed: %s", strings.Join(args, " "), output)

	return strings.TrimSpace(output)
}
