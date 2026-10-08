// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"libvirt.org/go/libvirtxml"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
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

// TestDomainDefinition verifies the running/stopped lifecycle of transient domains.
func (suite *LibvirtSuite) TestDomainDefinition() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)
	name := "vm-define-" + uuid.NewString()

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")

	// Register cleanup before applying configuration: a failed apply can still
	// leave a guest behind. Do not reuse the test's expiring context.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertStoppedDomain(nodeCtx, node, name)
	suite.assertConsoleRejected(nodeCtx, name, codes.FailedPrecondition)

	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertRunningTransientDomain(node, name, 1)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, name,
		func(spec *hypervisor.VirtualMachineDomainSpec, asrt *assert.Assertions) {
			asrt.True(spec.Metadata().Finalizers().Has("hypervisor.VirtualMachineController"))
		},
	)

	doc.CPUConfig.CPUCount = 2
	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertRunningTransientDomain(node, name, 2)

	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertStoppedDomain(nodeCtx, node, name)

	suite.RemoveMachineConfigDocumentsByName(nodeCtx, hypervisorcfg.VirtualMachineConfigKind, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainSpec](nodeCtx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineStatus](nodeCtx, suite.T(), suite.Client.COSI, name)

	// No status exists after document removal; inspect libvirt to catch an orphaned domain.
	suite.assertNoDomain(node, name)
}

// TestConsoleAttach verifies a live console session owns the managed VM's libvirt console.
func (suite *LibvirtSuite) TestConsoleAttach() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)
	name := "vm-console-" + uuid.NewString()

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)

	// Register cleanup before applying configuration: a failed apply can still
	// leave a guest behind. Do not reuse the test's expiring context.
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertRunningTransientDomain(node, name, 1)

	streamCtx, cancelStream := context.WithCancel(nodeCtx)
	defer cancelStream()

	stream, err := suite.Client.HypervisorClient.ConsoleStream(streamCtx)
	suite.Require().NoError(err)
	suite.Require().NoError(stream.Send(&machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_Attach{
			Attach: &machine.ConsoleAttach{Name: name},
		},
	}))

	recvResult := make(chan error, 1)

	go func() {
		for {
			if _, recvErr := stream.Recv(); recvErr != nil {
				recvResult <- recvErr

				return
			}
		}
	}()

	suite.assertLibvirtConsoleAttached(nodeCtx, node, name, recvResult)

	cancelStream()

	select {
	case recvErr := <-recvResult:
		suite.Require().Equal(codes.Canceled, status.Code(recvErr), "console receive after detach returned %v", recvErr)
	case <-time.After(10 * time.Second):
		suite.Require().FailNow("console receive did not stop after detach")
	}
}

// TestVNCAttach verifies machined can connect to the guest's sVirt-labeled VNC socket.
func (suite *LibvirtSuite) TestVNCAttach() {
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)
	name := "vm-vnc-" + uuid.NewString()

	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-virtqemud": "Running"})

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateRunning
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.ConsoleConfig.VNCConfig.VNCEnabled = new(true)

	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		suite.RemoveMachineConfigDocumentsByName(client.WithNode(ctx, node), hypervisorcfg.VirtualMachineConfigKind, name)
	})

	suite.PatchMachineConfig(nodeCtx, doc)
	suite.assertVirtualMachineStatus(nodeCtx, name, hypervisor.VirtualMachinePowerStateRunning)

	ctx, cancel := context.WithTimeout(nodeCtx, 10*time.Second)
	defer cancel()

	stream, err := suite.Client.HypervisorClient.VNCStream(ctx)
	suite.Require().NoError(err)
	suite.Require().NoError(stream.Send(&machine.VNCRequest{
		Request: &machine.VNCRequest_Attach{
			Attach: &machine.VNCAttach{Name: name},
		},
	}))

	const version = "RFB 003.008\n"

	var greeting []byte

	for len(greeting) < len(version) {
		response, recvErr := stream.Recv()
		suite.Require().NoError(recvErr, "VNC attachment must reach the guest socket and receive its RFB greeting")

		greeting = append(greeting, response.GetData()...)
	}

	suite.Require().Equal(version, string(greeting))
}

func (suite *LibvirtSuite) assertStoppedDomain(ctx context.Context, node, name string) {
	suite.T().Helper()

	// A missing domain status alone cannot distinguish a stopped VM from an unavailable daemon.
	suite.assertNoDomain(node, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineDomainStatus](ctx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(status *hypervisor.VirtualMachineStatus, asrt *assert.Assertions) {
			asrt.Equal(hypervisor.VirtualMachinePowerStateUnknown, status.TypedSpec().PowerState)
			asrt.Equal(hypervisor.VirtualMachineStageUnknown, status.TypedSpec().Stage)
		},
	)
}

func (suite *LibvirtSuite) assertVirtualMachineStatus(ctx context.Context, name string, powerState hypervisor.VirtualMachinePowerState) {
	suite.T().Helper()

	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(status *hypervisor.VirtualMachineStatus, asrt *assert.Assertions) {
			asrt.Equal(powerState, status.TypedSpec().PowerState)
			asrt.Equal(hypervisor.VirtualMachineStageReady, status.TypedSpec().Stage)
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

func (suite *LibvirtSuite) assertLibvirtConsoleAttached(ctx context.Context, node, name string, recvResult <-chan error) {
	suite.T().Helper()

	domainXML := suite.runVirsh(node, "dumpxml", name)

	var domain libvirtxml.Domain

	suite.Require().NoError(domain.Unmarshal(domainXML))
	suite.Require().NotNil(domain.Devices)
	suite.Require().NotEmpty(domain.Devices.Serials)
	suite.Require().NotNil(domain.Devices.Serials[0].Source)
	suite.Require().NotNil(domain.Devices.Serials[0].Source.Pty)

	ptyPath := domain.Devices.Serials[0].Source.Pty.Path
	suite.Require().NotEmpty(ptyPath)

	virtqemudPID, err := safe.ReaderGetByID[*runtime.ServicePID](ctx, suite.Client.COSI, "ext-virtqemud")
	suite.Require().NoError(err)

	lockPath := "/var/run/lock/LCK.." + strings.ReplaceAll(strings.TrimPrefix(ptyPath, "/dev"), "/", "_")
	busybox := "/nix/var/nix/profiles/default/bin/busybox"
	check := `
if ! test -d /var/run/lock; then
  echo "/var/run/lock is missing"
  exit 2
fi
if ! test -f "$1"; then
  exit 1
fi
if ! test "$("$4" cat "$1")" -eq "$2"; then
  echo "console lock $1 is not owned by virtqemud PID $2"
  exit 3
fi
for fd in /proc/"$2"/fd/*; do
  if test "$("$4" readlink "$fd")" = "$3"; then
    exit 0
  fi
done
exit 1
`

	checkCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	for {
		select {
		case err := <-recvResult:
			suite.Require().FailNow("console attach ended before libvirt observed it", "receive returned %v", err)
		default:
		}

		output, exitCode := suite.RunDebugContainer(checkCtx, node,
			busybox, "sh", "-c", check, "check-console", lockPath,
			fmt.Sprintf("%d", virtqemudPID.TypedSpec().PID), ptyPath, busybox,
		)
		if exitCode == 0 {
			return
		}

		if exitCode == 2 || exitCode == 3 {
			suite.Require().FailNow("invalid libvirt console lock", "exit code %d, output: %s", exitCode, output)
		}

		select {
		case err := <-recvResult:
			suite.Require().FailNow("console attach ended before libvirt observed it", "receive returned %v", err)
		case <-checkCtx.Done():
			suite.Require().FailNow("libvirt did not open the VM console", "lock %q, PTY %q, last exit code %d, output: %s", lockPath, ptyPath, exitCode, output)
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func (suite *LibvirtSuite) assertRunningTransientDomain(node, name string, cpus uint) {
	suite.T().Helper()

	suite.assertRunningTransientDomainWithDevices(node, name, cpus, 0, 0)
}

func (suite *LibvirtSuite) assertRunningTransientDomainWithDevices(node, name string, cpus uint, disks, interfaces int) {
	suite.T().Helper()

	suite.assertVirtualMachineStatus(client.WithNode(suite.ctx, node), name, hypervisor.VirtualMachinePowerStateRunning)

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
	suite.assertConsoleRejected(nodeCtx, libvirtDomainName, codes.NotFound)

	pidBeforeReboot, err := suite.libvirtDomainPID(nodeCtx)
	suite.Require().NoError(err)
	suite.Require().NotZero(pidBeforeReboot, "expected QEMU to run domain %q", libvirtDomainName)

	if testing.Short() {
		suite.T().Skip("skipping service restart and reboot checks in short mode")
	}

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

	suite.Require().Eventually(func() bool {
		pidAfterServiceRestart, err := suite.libvirtDomainPID(nodeCtx)

		return err == nil && pidAfterServiceRestart == pidBeforeReboot
	}, 30*time.Second, 100*time.Millisecond, "service restart must preserve the QEMU process with PID %d", pidBeforeReboot)

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

func (suite *LibvirtSuite) assertConsoleRejected(ctx context.Context, name string, code codes.Code) {
	suite.T().Helper()

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	stream, err := suite.Client.HypervisorClient.ConsoleStream(streamCtx)
	suite.Require().NoError(err)
	suite.Require().NoError(stream.Send(&machine.ConsoleRequest{
		Request: &machine.ConsoleRequest_Attach{
			Attach: &machine.ConsoleAttach{
				Name: name,
			},
		},
	}))

	_, err = stream.Recv()
	suite.Require().Equal(code, status.Code(err), "console attach for %q returned %v", name, err)
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
