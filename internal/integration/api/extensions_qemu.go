// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
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

// TestExtensionsKataContainers verifies gvisor runtime class is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsKataContainers() {
	suite.testRuntimeClass("kata", "kata")
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

// TestExtensionsMdADM verifies mdadm is working, udev rules work and the raid is mounted on reboot.
func (suite *ExtensionsSuiteQEMU) TestExtensionsMdADM() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	userDisks := suite.UserDisks(suite.ctx, node)

	suite.Require().GreaterOrEqual(len(userDisks), 2, "expected at least two user disks to be available")

	raidDisks := userDisks[:2]

	stdout, exitCode, err := suite.ExecInHostMountNS(suite.ctx, node,
		append([]string{"mdadm", "--create", "/dev/md/testmd", "--raid-devices=2", "--metadata=1.2", "--level=1"}, raidDisks...)...,
	)
	suite.Require().NoError(err)
	suite.Require().EqualValues(0, exitCode, "mdadm --create failed: %s", stdout)

	suite.Require().Contains(stdout, "mdadm: array /dev/md/testmd started.")

	defer func() {
		hostNameStatus, err := safe.StateGetByID[*network.HostnameStatus](client.WithNode(suite.ctx, node), suite.Client.COSI, "hostname")
		suite.Require().NoError(err)

		hostname := hostNameStatus.TypedSpec().Hostname

		if _, _, err := suite.ExecInHostMountNS(suite.ctx, node,
			"mdadm", "--wait", "--stop", "/dev/md/"+hostname+":testmd",
		); err != nil {
			suite.T().Logf("failed to stop mdadm array: %v", err)
		}

		if _, _, err := suite.ExecInHostMountNS(suite.ctx, node,
			append([]string{"mdadm", "--zero-superblock"}, raidDisks...)...,
		); err != nil {
			suite.T().Logf("failed to remove md array backed by volumes %v: %v", raidDisks, err)
		}
	}()

	// now we want to reboot the node and make sure the array is still mounted
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
	)

	suite.Require().True(suite.mdADMArrayExists(), "expected mdadm array to be present")
}

func (suite *ExtensionsSuiteQEMU) mdADMArrayExists() bool {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	ctx := client.WithNode(suite.ctx, node)

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	for disk := range disks.All() {
		if strings.HasPrefix(disk.TypedSpec().DevPath, "/dev/md") {
			return true
		}
	}

	return false
}

// TestExtensionsZFS verifies zfs is working, udev rules work and the pool is mounted on reboot.
func (suite *ExtensionsSuiteQEMU) TestExtensionsZFS() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-zfs-service": "Running"})

	userDisks := suite.UserDisks(suite.ctx, node)

	suite.Require().NotEmpty(userDisks, "expected at least one user disks to be available")

	stdout, exitCode, err := suite.ExecInHostMountNS(suite.ctx, node,
		"zpool", "create", "-m", "/var/tank", "tank", userDisks[0],
	)
	suite.Require().NoError(err)
	suite.Require().EqualValues(0, exitCode, "zpool create failed: %s", stdout)
	suite.Require().Equal("", stdout)

	stdout, exitCode, err = suite.ExecInHostMountNS(suite.ctx, node,
		"zfs", "create", "-V", "1gb", "tank/vol",
	)
	suite.Require().NoError(err)
	suite.Require().EqualValues(0, exitCode, "zfs create failed: %s", stdout)
	suite.Require().Equal("", stdout)

	defer func() {
		if _, _, err := suite.ExecInHostMountNS(suite.ctx, node, "zfs", "destroy", "tank/vol"); err != nil {
			suite.T().Logf("failed to remove zfs dataset tank/vol: %v", err)
		}

		if _, _, err := suite.ExecInHostMountNS(suite.ctx, node, "zpool", "destroy", "tank"); err != nil {
			suite.T().Logf("failed to remove zpool tank: %v", err)
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

	stdout, exitCode, err := suite.ExecInHostMountNS(suite.ctx, node,
		"/usr/local/sbin/fstrim", "--version",
	)
	suite.Require().NoError(err)
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
