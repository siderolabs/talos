// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const (
	libvirtDomainName = "talos-integration-libvirt"
	libvirtURI        = "qemu+unix:///system?socket=/run/libvirt/virtqemud-sock"
)

// LibvirtSuite verifies the libvirt extension on QEMU.
type LibvirtSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *LibvirtSuite) SuiteName() string {
	return "hypervisor.LibvirtSuite"
}

// SetupTest ...
func (suite *LibvirtSuite) SetupTest() {
	if !suite.ExtensionsLibvirt {
		suite.T().Skip("skipping as libvirt extension integration tests are not enabled")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboot")
	}

	// Allow time for domain operations as well as a node reboot.
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Minute)
}

// TearDownTest ...
func (suite *LibvirtSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestDomain verifies libvirt can run a QEMU domain and save it across a reboot.
func (suite *LibvirtSuite) TestDomain() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{
		"ext-virtlockd":    "Running",
		"ext-virtlogd":     "Running",
		"ext-virtqemud":    "Running",
		"ext-virtstoraged": "Running",
	})

	var (
		machineType string
		qemuArch    string
	)

	switch arch := suite.ReadMachineArch(nodeCtx); arch {
	case "amd64":
		machineType = "pc"
		qemuArch = "x86_64"
	case "arm64":
		machineType = "virt"
		qemuArch = "aarch64"
	default:
		suite.Require().FailNow("unsupported architecture", "architecture %q is not supported by the libvirt integration test", arch)
	}

	domainXMLPath := "/var/lib/libvirt/" + libvirtDomainName + ".xml"
	domainXML := fmt.Sprintf(`<domain type="kvm">
  <name>%s</name>
  <memory unit="MiB">128</memory>
  <vcpu placement="static">1</vcpu>
  <os>
    <type arch="%s" machine="%s">hvm</type>
  </os>
  <devices>
    <emulator>/usr/local/bin/qemu-system-%s</emulator>
  </devices>
</domain>
`, libvirtDomainName, qemuArch, machineType, qemuArch)

	writeDomainXML := fmt.Sprintf(
		"/nix/var/nix/profiles/default/bin/busybox echo %s | "+
			"/nix/var/nix/profiles/default/bin/busybox base64 -d > %s",
		base64.StdEncoding.EncodeToString([]byte(domainXML)), domainXMLPath,
	)
	output, exitCode := suite.RunDebugContainer(suite.ctx, node, "/nix/var/nix/profiles/default/bin/sh", "-c", writeDomainXML)
	suite.Require().EqualValues(0, exitCode, "failed to write libvirt domain XML: %s", output)

	defer func() {
		cleanup := fmt.Sprintf(
			"/usr/local/bin/virsh --connect '%s' destroy %s >/dev/null 2>&1 || true; "+
				"/usr/local/bin/virsh --connect '%s' undefine %s --managed-save >/dev/null 2>&1 || "+
				"/usr/local/bin/virsh --connect '%s' undefine %s >/dev/null 2>&1 || true; "+
				"/nix/var/nix/profiles/default/bin/busybox rm -f %s",
			libvirtURI, libvirtDomainName,
			libvirtURI, libvirtDomainName,
			libvirtURI, libvirtDomainName,
			domainXMLPath,
		)

		cleanupOutput, cleanupExitCode := suite.RunDebugContainer(suite.ctx, node, "/nix/var/nix/profiles/default/bin/sh", "-c", cleanup)
		if cleanupExitCode != 0 {
			suite.T().Logf("failed to clean up libvirt domain: %s", cleanupOutput)
		}
	}()

	suite.runVirsh(node, "define", domainXMLPath)
	suite.runVirsh(node, "start", libvirtDomainName)
	suite.Require().Equal("running", suite.runVirsh(node, "domstate", libvirtDomainName))

	pidBeforeReboot, err := suite.libvirtDomainPID(nodeCtx)
	suite.Require().NoError(err)
	suite.Require().NotZero(pidBeforeReboot, "expected QEMU to run domain %q", libvirtDomainName)

	virtqemudPID, err := safe.ReaderGetByID[*runtime.ServicePID](nodeCtx, suite.Client.COSI, "ext-virtqemud")
	suite.Require().NoError(err)

	_, err = suite.Client.ServiceRestart(nodeCtx, "ext-virtqemud")
	suite.Require().NoError(err)

	rtestutils.AssertResource(
		nodeCtx, suite.T(), suite.Client.COSI,
		"ext-virtqemud",
		func(servicePID *runtime.ServicePID, asrt *assert.Assertions) {
			asrt.NotEqual(virtqemudPID.TypedSpec().PID, servicePID.TypedSpec().PID)
		},
	)

	suite.Require().Equal("running", suite.runVirsh(node, "domstate", libvirtDomainName))

	pidAfterServiceRestart, err := suite.libvirtDomainPID(nodeCtx)
	suite.Require().NoError(err)
	suite.Require().Equal(pidBeforeReboot, pidAfterServiceRestart, "service restart must preserve the QEMU process")

	// Node-level reboot check: this suite runs without Kubernetes.
	suite.AssertRebootedNoChecks(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
	)

	suite.WaitForBootDone(suite.ctx)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})
	suite.Require().Regexp(`(?m)^Managed save:\s+yes$`, suite.runVirsh(node, "dominfo", libvirtDomainName))

	pidAfterReboot, err := suite.libvirtDomainPID(nodeCtx)
	suite.Require().NoError(err)
	suite.Require().Zero(pidAfterReboot, "managed-saved domain must not have a running QEMU process")

	suite.runVirsh(node, "start", libvirtDomainName)
	suite.Require().Equal("running", suite.runVirsh(node, "domstate", libvirtDomainName))
	suite.Require().Regexp(`(?m)^Managed save:\s+no$`, suite.runVirsh(node, "dominfo", libvirtDomainName))

	pidAfterRestore, err := suite.libvirtDomainPID(nodeCtx)
	suite.Require().NoError(err)
	suite.Require().NotZero(pidAfterRestore, "expected QEMU to restore domain %q", libvirtDomainName)
}

func (suite *LibvirtSuite) runVirsh(node string, args ...string) string {
	command := append([]string{"/usr/local/bin/virsh", "--connect", libvirtURI}, args...)
	output, exitCode := suite.RunDebugContainer(suite.ctx, node, command...)
	suite.Require().EqualValues(0, exitCode, "virsh %s failed: %s", strings.Join(args, " "), output)

	return strings.TrimSpace(output)
}

func (suite *LibvirtSuite) libvirtDomainPID(ctx context.Context) (int32, error) {
	response, err := suite.Client.Processes(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list processes: %w", err)
	}

	for _, message := range response.Messages {
		for _, process := range message.Processes {
			if strings.Contains(process.Executable, "/qemu-system-") && strings.Contains(process.Args, "guest="+libvirtDomainName+",") {
				return process.Pid, nil
			}
		}
	}

	return 0, nil
}

func init() {
	allSuites = append(allSuites, &LibvirtSuite{})
}
