// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/api/storage"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ExtensionsSuiteQEMU verifies Talos extensions on QEMU.
type ExtensionsSuiteQEMU struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ExtensionsSuiteQEMU) SuiteName() string {
	return "api.ExtensionsSuiteQEMU"
}

// SetupTest ...
func (suite *ExtensionsSuiteQEMU) SetupTest() {
	if !suite.ExtensionsQEMU {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *ExtensionsSuiteQEMU) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestExtensionsExpectedPaths verifies expected paths are present.
func (suite *ExtensionsSuiteQEMU) TestExtensionsExpectedPaths() {
	expectedPaths := []string{
		"/lib/firmware/amdgpu",
		"/lib/firmware/amd-ucode",
		"/lib/firmware/bnx2x",
		"/lib/firmware/cxgb3",
		"/lib/firmware/cxgb4/configs",
		"/lib/firmware/i915",
		"/lib/firmware/intel/ice/ddp",
		"/lib/firmware/intel-ucode",
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	for _, path := range expectedPaths {
		stream, err := suite.Client.LS(ctx, &machineapi.ListRequest{
			Root:  path,
			Types: []machineapi.ListRequest_Type{machineapi.ListRequest_DIRECTORY},
		})

		suite.Require().NoError(err)

		suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
			suite.Require().Equal(path, info.Name, "expected %s to exist", path)

			return nil
		}))
	}
}

// TestExtensionsExpectedModules verifies expected modules are loaded and in modules.dep.
func (suite *ExtensionsSuiteQEMU) TestExtensionsExpectedModules() {
	expectedModules := []string{
		"asix",
		"ax88179_178a",
		"ax88796b",
		"binfmt_misc",
		"btrfs",
		"cdc_ether",
		"cdc_mbim",
		"cdc_ncm",
		"cdc_subset",
		"cdc_wdm",
		"cxgb",
		"cxgb3",
		"cxgb4",
		"cxgb4vf",
		"drbd",
		"ena",
		"gasket",
		"net1080",
		"option",
		"qmi_wwan",
		"r8153_ecm",
		"thunderbolt",
		"thunderbolt_net",
		"usb_wwan",
		"usbnet",
		"xdma",
		"zaurus",
		"zfs",
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertExpectedModules(suite.ctx, node, expectedModules)
}

// TestExtensionsNutClient verifies nut client is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsNutClient() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-nut-client": "Running"})
}

// TestExtensionsQEMUGuestAgent verifies qemu guest agent is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsQEMUGuestAgent() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-qemu-guest-agent": "Running"})

	ctx := client.WithNode(suite.ctx, node)

	hostnameSpec, err := safe.StateWatchFor[*network.HostnameStatus](
		ctx,
		suite.Client.COSI,
		network.NewHostnameStatus(network.NamespaceName, resource.ID("hostname")).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
	)
	suite.Require().NoError(err)

	clusterStatePath, err := suite.Cluster.StatePath()
	suite.Require().NoError(err)

	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", filepath.Join(clusterStatePath, hostnameSpec.TypedSpec().Hostname+".sock"))
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	// now we want to reboot the node using the guest agent
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			_, err = conn.Write([]byte(`{"execute":"guest-shutdown", "arguments": {"mode": "reboot"}}`))

			return err
		}, 5*time.Minute,
		suite.CleanupFailedPods,
	)
}

const (
	libvirtDomainName = "talos-integration-libvirt"
	libvirtURI        = "qemu+unix:///system?socket=/run/libvirt/virtqemud-sock"
)

// TestExtensionsLibvirt verifies libvirt can run a QEMU domain and save it across a reboot.
func (suite *ExtensionsSuiteQEMU) TestExtensionsLibvirt() {
	if !suite.ExtensionsLibvirt {
		suite.T().Skip("skipping as libvirt extension integration tests are not enabled")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
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

	suite.AssertRebooted(
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

func (suite *ExtensionsSuiteQEMU) runVirsh(node string, args ...string) string {
	command := append([]string{"/usr/local/bin/virsh", "--connect", libvirtURI}, args...)
	output, exitCode := suite.RunDebugContainer(suite.ctx, node, command...)
	suite.Require().EqualValues(0, exitCode, "virsh %s failed: %s", strings.Join(args, " "), output)

	return strings.TrimSpace(output)
}

func (suite *ExtensionsSuiteQEMU) libvirtDomainPID(ctx context.Context) (int32, error) {
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

// TestExtensionsTailscale verifies tailscale is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsTailscale() {
	// Tailscale service keeps on restarting unless authed, so this test is disabled for now.
	if ok := os.Getenv("TALOS_INTEGRATION_RUN_TAILSCALE"); ok == "" {
		suite.T().Skip("skipping as tailscale integration tests are not enabled")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-tailscale": "Running"})

	ctx := client.WithNode(suite.ctx, node)

	linkSpec, err := safe.StateWatchFor[*network.LinkStatus](
		ctx,
		suite.Client.COSI,
		network.NewHostnameStatus(network.NamespaceName, resource.ID("tailscale0")).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
	)
	suite.Require().NoError(err)

	suite.Require().Equal("tun", linkSpec.TypedSpec().Kind)
}

// TestExtensionsHelloWorldService verifies hello world service is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsHelloWorldService() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{
		"ext-hello-world": "Running",
	})

	url := url.URL{
		Scheme: "http",
		Host:   node,
	}

	resp, err := http.Get(url.String()) //nolint:noctx
	suite.Require().NoError(err)

	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	suite.Require().NoError(err)

	suite.Require().Equal("Hello from Talos Linux Extension Service!", string(respBody))
}

// TestExtensionsGvisor verifies gvisor runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsGvisor() {
	suite.testRuntimeClass("gvisor", "runsc")
}

// TestExtensionsGvisorKVM verifies gvisor runtime class with kvm platform is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsGvisorKVM() {
	suite.testRuntimeClass("gvisor-kvm", "runsc-kvm")
}

// TestExtensionsCrun verifies crun runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsCrun() {
	suite.testRuntimeClass("crun", "crun")
}

// TestExtensionsKataContainers verifies that Kata Containers Cloud Hypervisor runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsKataContainers() {
	suite.testRuntimeClass("kata", "kata")
}

// TestExtensionsKataContainersQEMU verifies that Kata Containers QEMU runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsKataContainersQEMU() {
	suite.testRuntimeClass("kata-qemu", "kata-qemu")
}

// TestExtensionsKataContainersSNP verifies that Kata Containers confidential VMs runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsKataContainersSNP() {
	suite.testRuntimeClass("kata-qemu-coco-dev", "kata-qemu-coco-dev")
}

// TestExtensionsYouki verifies youki runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsYouki() {
	suite.testRuntimeClass("youki", "youki")
}

func (suite *ExtensionsSuiteQEMU) testRuntimeClass(runtimeClassName, handlerName string) {
	testName := "nginx-" + runtimeClassName

	_, err := suite.Clientset.NodeV1().RuntimeClasses().Create(suite.ctx, &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: runtimeClassName,
		},
		Handler: handlerName,
	}, metav1.CreateOptions{})
	if apierrors.IsAlreadyExists(err) {
		// ignore if the runtime class already exists
		err = nil
	}

	suite.Require().NoError(err)

	defer suite.Clientset.NodeV1().RuntimeClasses().Delete(suite.ctx, runtimeClassName, metav1.DeleteOptions{}) //nolint:errcheck

	_, err = suite.Clientset.CoreV1().Pods("default").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: new(runtimeClassName),
			Containers: []corev1.Container{
				{
					Name:  testName,
					Image: "nginx",
				},
			},
		},
	}, metav1.CreateOptions{})
	suite.Require().NoError(err)

	defer suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, testName, metav1.DeleteOptions{}) //nolint:errcheck

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "default", testName))
}

// TestExtensionsStargz verifies stargz snapshotter.
func (suite *ExtensionsSuiteQEMU) TestExtensionsStargz() {
	_, err := suite.Clientset.CoreV1().Pods("default").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "stargz-hello",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "stargz-hello",
					Image: "ghcr.io/stargz-containers/alpine:3.15.3-esgz",
					Args:  []string{"sleep", "inf"},
				},
			},
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, "stargz-hello", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "default", "stargz-hello"))
}

// TestExtensionsZFS verifies zfs is working, udev rules work and the pool is mounted on reboot.
func (suite *ExtensionsSuiteQEMU) TestExtensionsZFS() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-zfs-service": "Running"})

	userDisks := suite.UserDisks(suite.ctx, node)

	suite.Require().NotEmpty(userDisks, "expected at least one user disks to be available")

	stdout, exitCode := suite.RunDebugContainer(suite.ctx, node,
		"zpool", "create", "-m", "/var/tank", "tank", userDisks[0],
	)
	suite.Require().EqualValues(0, exitCode, "zpool create failed: %s", stdout)
	suite.Require().Equal("", stdout)

	stdout, exitCode = suite.RunDebugContainer(suite.ctx, node,
		"zfs", "create", "-V", "1gb", "tank/vol",
	)
	suite.Require().EqualValues(0, exitCode, "zfs create failed: %s", stdout)
	suite.Require().Equal("", stdout)

	defer func() {
		suite.RunDebugContainer(suite.ctx, node, "zfs", "destroy", "tank/vol")

		suite.RunDebugContainer(suite.ctx, node, "zpool", "destroy", "tank")

		// Wipe the disk so no zfs label lingers (otherwise the pool is re-discovered
		// as a volume after the test).
		if err := suite.Client.BlockDeviceWipe(client.WithNode(suite.ctx, node), &storage.BlockDeviceWipeRequest{
			Devices: []*storage.BlockDeviceWipeDescriptor{{Device: filepath.Base(userDisks[0])}},
		}); err != nil {
			suite.T().Logf("failed to wipe disk %s: %v", userDisks[0], err)
		}
	}()

	suite.EventuallyWithT(func(t *assert.CollectT) {
		suite.checkZFSPoolMounted(t, node)
	}, 2*time.Minute, time.Second, "expected zfs pool to be mounted")

	// now we want to reboot the node and make sure the pool is still mounted
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
	)

	suite.EventuallyWithT(func(t *assert.CollectT) {
		suite.checkZFSPoolMounted(t, node)
	}, 30*time.Second, time.Second, "expected zfs pool to be mounted after reboot")
}

func (suite *ExtensionsSuiteQEMU) checkZFSPoolMounted(t *assert.CollectT, node string) {
	ctx := client.WithNode(suite.ctx, node)

	stream, err := suite.Client.LS(ctx, &machineapi.ListRequest{
		Root:  "/dev/zvol/tank/",
		Types: []machineapi.ListRequest_Type{machineapi.ListRequest_SYMLINK},
	})
	if !assert.NoError(t, err, "LS /dev/zvol/tank/") {
		return
	}

	found := false

	if !assert.NoError(t, helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
		if info.Name == "/dev/zvol/tank/vol" && strings.HasPrefix(filepath.Base(info.Link), "zd") {
			found = true
		}

		return nil
	}), "reading LS stream") {
		return
	}

	assert.True(t, found, "expected /dev/zvol/tank/vol symlink pointing to a zd* device")

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	if !assert.NoError(t, err, "StateListAll disks") {
		return
	}

	for disk := range disks.All() {
		if strings.HasPrefix(disk.TypedSpec().DevPath, "/dev/zd") {
			return
		}
	}

	assert.Fail(t, "no /dev/zd* disk found in block resources")
}

// TestExtensionsUtilLinuxTools verifies util-linux-tools are working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsUtilLinuxTools() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	stdout, exitCode := suite.RunDebugContainer(suite.ctx, node,
		"/usr/local/sbin/fstrim", "--version",
	)
	suite.Require().EqualValues(0, exitCode, "fstrim --version failed: %s", stdout)
	suite.Require().Contains(stdout, "fstrim from util-linux")
}

// TestExtensionsWasmEdge verifies wasmedge runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsWasmEdge() {
	_, err := suite.Clientset.NodeV1().RuntimeClasses().Create(suite.ctx, &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wasmedge",
		},
		Handler: "wasmedge",
	}, metav1.CreateOptions{})
	defer suite.Clientset.NodeV1().RuntimeClasses().Delete(suite.ctx, "wasmedge", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	_, err = suite.Clientset.CoreV1().Pods("default").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wasmedge-test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "wasmedge-test",
					Image: "wasmedge/example-wasi:latest",
				},
			},
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, "wasmedge-test", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "default", "wasmedge-test"))
}

// TestExtensionsSpin verifies spin runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsSpin() {
	_, err := suite.Clientset.NodeV1().RuntimeClasses().Create(suite.ctx, &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "wasmtime-spin-v2",
		},
		Handler: "spin",
	}, metav1.CreateOptions{})
	defer suite.Clientset.NodeV1().RuntimeClasses().Delete(suite.ctx, "wasmtime-spin-v2", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	_, err = suite.Clientset.CoreV1().Pods("default").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "spin-test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "spin-test",
					Image:   "ghcr.io/spinkube/containerd-shim-spin/examples/spin-rust-hello",
					Command: []string{"/"},
				},
			},
			RuntimeClassName: new("wasmtime-spin-v2"),
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, "spin-test", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "default", "spin-test"))
}

// TestLoadedKernelModule tests the /proc/modules resource.
func (suite *ExtensionsSuiteQEMU) TestLoadedKernelModule() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	suite.T().Logf("using node %s", node)

	rtestutils.AssertResources(
		ctx, suite.T(), suite.Client.COSI, []resource.ID{
			"virtio_balloon",
			"virtio_pci",
			"virtio_pci_legacy_dev",
			"virtio_pci_modern_dev",
		},
		func(res *runtime.LoadedKernelModule, asrt *assert.Assertions) { //nolint:staticcheck
			asrt.NotEmpty(res.TypedSpec().Size, "kernel module size should not be empty")
			asrt.NotEmpty(res.TypedSpec().Address, "kernel module address should not be empty")
			asrt.GreaterOrEqual(res.TypedSpec().ReferenceCount, 0, "kernel module instances should be non-negative")
		},
	)
}

func init() {
	allSuites = append(allSuites, &ExtensionsSuiteQEMU{})
}
