// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
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
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
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
	// expectedModulesModDep is a map of module name to module.dep name
	expectedModulesModDep := map[string]string{
		"asix":            "asix.ko",
		"ax88179_178a":    "ax88179_178a.ko",
		"ax88796b":        "ax88796b.ko",
		"binfmt_misc":     "binfmt_misc.ko",
		"btrfs":           "btrfs.ko",
		"cdc_ether":       "cdc_ether.ko",
		"cdc_mbim":        "cdc_mbim.ko",
		"cdc_ncm":         "cdc_ncm.ko",
		"cdc_subset":      "cdc_subset.ko",
		"cdc_wdm":         "cdc-wdm.ko",
		"cxgb":            "cxgb.ko",
		"cxgb3":           "cxgb3.ko",
		"cxgb4":           "cxgb4.ko",
		"cxgb4vf":         "cxgb4vf.ko",
		"drbd":            "drbd.ko",
		"gasket":          "gasket.ko",
		"net1080":         "net1080.ko",
		"option":          "option.ko",
		"qmi_wwan":        "qmi_wwan.ko",
		"r8153_ecm":       "r8153_ecm.ko",
		"thunderbolt":     "thunderbolt.ko",
		"thunderbolt_net": "thunderbolt_net.ko",
		"usb_wwan":        "usb_wwan.ko",
		"usbnet":          "usbnet.ko",
		"zaurus":          "zaurus.ko",
		"zfs":             "zfs.ko",
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertExpectedModules(suite.ctx, node, expectedModulesModDep)
}

// TestExtensionsISCSI verifies expected services are running.
func (suite *ExtensionsSuiteQEMU) TestExtensionsISCSI() {
	expectedServices := map[string]string{
		"ext-iscsid": "Running",
		"ext-tgtd":   "Running",
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertServicesRunning(suite.ctx, node, expectedServices)

	ctx := client.WithNode(suite.ctx, node)

	iscsiCreatePodDef, err := suite.NewPrivilegedPod("iscsi-create")
	suite.Require().NoError(err)

	suite.Require().NoError(iscsiCreatePodDef.Create(suite.ctx, 5*time.Minute))

	defer iscsiCreatePodDef.Delete(suite.ctx) //nolint:errcheck

	reader, err := suite.Client.Read(ctx, "/system/iscsi/initiatorname.iscsi")
	suite.Require().NoError(err)

	defer reader.Close() //nolint:errcheck

	body, err := io.ReadAll(reader)
	suite.Require().NoError(err)

	initiatorName := strings.TrimPrefix(strings.TrimSpace(string(body)), "InitiatorName=")

	stdout, stderr, err := iscsiCreatePodDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- tgtadm --lld iscsi --op new --mode target --tid 1 -T %s", initiatorName),
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal("", stdout)

	stdout, stderr, err = iscsiCreatePodDef.Exec(
		suite.ctx,
		"dd if=/dev/zero of=/proc/$(pgrep tgtd)/root/var/run/tgtd/iscsi.disk bs=1M count=100",
	)
	suite.Require().NoError(err)

	suite.Require().Contains(stderr, "100+0 records in\n100+0 records out\n")
	suite.Require().Equal("", stdout)

	stdout, stderr, err = iscsiCreatePodDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- tgtadm --lld iscsi --op new --mode logicalunit --tid 1 --lun 1 -b /var/run/tgtd/iscsi.disk",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal("", stdout)

	stdout, stderr, err = iscsiCreatePodDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- tgtadm --lld iscsi --op bind --mode target --tid 1 -I ALL",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal("", stdout)

	stdout, stderr, err = iscsiCreatePodDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/$(pgrep iscsid)/ns/mnt --net=/proc/$(pgrep iscsid)/ns/net -- iscsiadm --mode discovery --type sendtargets --portal %s:3260", node),
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal(fmt.Sprintf("%s:3260,1 %s\n", node, initiatorName), stdout)

	stdout, stderr, err = iscsiCreatePodDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/$(pgrep iscsid)/ns/mnt --net=/proc/$(pgrep iscsid)/ns/net -- iscsiadm --mode node --targetname %s --portal %s:3260 --login", initiatorName, node),
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Contains(stdout, "successful.")

	defer func() {
		stdout, stderr, err = iscsiCreatePodDef.Exec(
			suite.ctx,
			fmt.Sprintf("nsenter --mount=/proc/$(pgrep iscsid)/ns/mnt --net=/proc/$(pgrep iscsid)/ns/net -- iscsiadm --mode node --targetname %s --portal %s:3260 --logout", initiatorName, node),
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)

		stdout, stderr, err = iscsiCreatePodDef.Exec(
			suite.ctx,
			"nsenter --mount=/proc/1/ns/mnt -- tgtadm --lld iscsi --op delete --mode logicalunit --tid 1 --lun 1",
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Equal("", stdout)

		stdout, stderr, err = iscsiCreatePodDef.Exec(
			suite.ctx,
			"nsenter --mount=/proc/1/ns/mnt -- tgtadm --lld iscsi --op delete --mode target --tid 1",
		)

		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Equal("", stdout)
	}()

	suite.Eventually(func() bool {
		return suite.iscsiTargetExists()
	}, 5*time.Second, 1*time.Second, "expected iscsi target to exist")
}

func (suite *ExtensionsSuiteQEMU) iscsiTargetExists() bool {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	ctx := client.WithNode(suite.ctx, node)

	disks, err := safe.ReaderListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	for disk := range disks.All() {
		if disk.TypedSpec().Transport == "iscsi" {
			return true
		}
	}

	return false
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

	conn, err := net.Dial("unix", filepath.Join(clusterStatePath, hostnameSpec.TypedSpec().Hostname+".sock"))
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	// now we want to reboot the node using the guest agent
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			_, err = conn.Write([]byte(`{"execute":"guest-shutdown", "arguments": {"mode": "reboot"}}`))

			return err
		}, 5*time.Minute,
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
	suite.T().Skip("skipping until https://github.com/siderolabs/extensions/issues/417 is addressed.")

	suite.testRuntimeClass("gvisor", "runsc")
}

// TestExtensionsGvisorKVM verifies gvisor runtime class with kvm platform is working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsGvisorKVM() {
	suite.T().Skip("skipping until https://github.com/siderolabs/extensions/issues/417 is addressed.")

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

func (suite *ExtensionsSuiteQEMU) testRuntimeClass(runtimeClassName, handlerName string) {
	testName := "nginx-" + runtimeClassName

	_, err := suite.Clientset.NodeV1().RuntimeClasses().Create(suite.ctx, &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: runtimeClassName,
		},
		Handler: handlerName,
	}, metav1.CreateOptions{})
	defer suite.Clientset.NodeV1().RuntimeClasses().Delete(suite.ctx, runtimeClassName, metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	_, err = suite.Clientset.CoreV1().Pods("default").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: testName,
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: pointer.To(runtimeClassName),
			Containers: []corev1.Container{
				{
					Name:  testName,
					Image: "nginx",
				},
			},
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, testName, metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

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

	userDisks, err := suite.UserDisks(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Require().GreaterOrEqual(len(userDisks), 2, "expected at least two user disks to be available")

	userDisksJoined := strings.Join(userDisks[:2], " ")

	mdAdmCreatePodDef, err := suite.NewPrivilegedPod("mdadm-create")
	suite.Require().NoError(err)

	suite.Require().NoError(mdAdmCreatePodDef.Create(suite.ctx, 5*time.Minute))

	defer mdAdmCreatePodDef.Delete(suite.ctx) //nolint:errcheck

	stdout, _, err := mdAdmCreatePodDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- mdadm --create /dev/md/testmd --raid-devices=2 --metadata=1.2 --level=1 %s", userDisksJoined),
	)
	suite.Require().NoError(err)

	suite.Require().Contains(stdout, "mdadm: array /dev/md/testmd started.")

	defer func() {
		hostNameStatus, err := safe.StateGetByID[*network.HostnameStatus](client.WithNode(suite.ctx, node), suite.Client.COSI, "hostname")
		suite.Require().NoError(err)

		hostname := hostNameStatus.TypedSpec().Hostname

		deletePodDef, err := suite.NewPrivilegedPod("mdadm-destroy")
		suite.Require().NoError(err)

		suite.Require().NoError(deletePodDef.Create(suite.ctx, 5*time.Minute))

		defer deletePodDef.Delete(suite.ctx) //nolint:errcheck

		if _, _, err := deletePodDef.Exec(
			suite.ctx,
			fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- mdadm --wait --stop /dev/md/%s:testmd", hostname),
		); err != nil {
			suite.T().Logf("failed to stop mdadm array: %v", err)
		}

		if _, _, err := deletePodDef.Exec(
			suite.ctx,
			fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- mdadm --zero-superblock %s", userDisksJoined),
		); err != nil {
			suite.T().Logf("failed to remove md array backed by volumes %s: %v", userDisksJoined, err)
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
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-zpool-importer": "Finished"})

	userDisks, err := suite.UserDisks(suite.ctx, node)
	suite.Require().NoError(err)

	suite.Require().NotEmpty(userDisks, "expected at least one user disks to be available")

	zfsPodDef, err := suite.NewPrivilegedPod("zpool-create")
	suite.Require().NoError(err)

	suite.Require().NoError(zfsPodDef.Create(suite.ctx, 5*time.Minute))

	defer zfsPodDef.Delete(suite.ctx) //nolint:errcheck

	stdout, stderr, err := zfsPodDef.Exec(
		suite.ctx,
		fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- zpool create -m /var/tank tank %s", userDisks[0]),
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal("", stdout)

	stdout, stderr, err = zfsPodDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- zfs create -V 1gb tank/vol",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().Equal("", stdout)

	defer func() {
		deletePodDef, err := suite.NewPrivilegedPod("zpool-destroy")
		suite.Require().NoError(err)

		suite.Require().NoError(deletePodDef.Create(suite.ctx, 5*time.Minute))

		defer deletePodDef.Delete(suite.ctx) //nolint:errcheck

		if _, _, err := deletePodDef.Exec(
			suite.ctx,
			"nsenter --mount=/proc/1/ns/mnt -- zfs destroy tank/vol",
		); err != nil {
			suite.T().Logf("failed to remove zfs dataset tank/vol: %v", err)
		}

		if _, _, err := deletePodDef.Exec(
			suite.ctx,
			"nsenter --mount=/proc/1/ns/mnt -- zpool destroy tank",
		); err != nil {
			suite.T().Logf("failed to remove zpool tank: %v", err)
		}
	}()

	suite.Require().True(suite.checkZFSPoolMounted(), "expected zfs pool to be mounted")

	// now we want to reboot the node and make sure the pool is still mounted
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
	)

	suite.Require().True(suite.checkZFSPoolMounted(), "expected zfs pool to be mounted")
}

func (suite *ExtensionsSuiteQEMU) checkZFSPoolMounted() bool {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	ctx := client.WithNode(suite.ctx, node)

	disks, err := safe.StateListAll[*block.Disk](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	for disk := range disks.All() {
		if strings.HasPrefix(disk.TypedSpec().DevPath, "/dev/zd") {
			return true
		}
	}

	return false
}

// TestExtensionsUtilLinuxTools verifies util-linux-tools are working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsUtilLinuxTools() {
	utilLinuxPodDef, err := suite.NewPrivilegedPod("util-linux-tools-test")
	suite.Require().NoError(err)

	suite.Require().NoError(utilLinuxPodDef.Create(suite.ctx, 5*time.Minute))

	defer utilLinuxPodDef.Delete(suite.ctx) //nolint:errcheck

	stdout, stderr, err := utilLinuxPodDef.Exec(
		suite.ctx,
		"nsenter --mount=/proc/1/ns/mnt -- /usr/local/sbin/fstrim --version",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
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
			RuntimeClassName: pointer.To("wasmtime-spin-v2"),
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, "spin-test", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "default", "spin-test"))
}

func init() {
	allSuites = append(allSuites, &ExtensionsSuiteQEMU{})
}
