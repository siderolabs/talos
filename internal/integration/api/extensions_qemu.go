// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"crypto/rand"
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
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ExtensionsSuiteQEMU verifies Talos is securebooted.
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
		"asix":         "asix.ko",
		"ax88179_178a": "ax88179_178a.ko",
		"ax88796b":     "ax88796b.ko",
		"binfmt_misc":  "binfmt_misc.ko",
		"btrfs":        "btrfs.ko",
		"cdc_ether":    "cdc_ether.ko",
		"cdc_mbim":     "cdc_mbim.ko",
		"cdc_ncm":      "cdc_ncm.ko",
		"cdc_subset":   "cdc_subset.ko",
		"cdc_wdm":      "cdc-wdm.ko",
		"cxgb":         "cxgb.ko",
		"cxgb3":        "cxgb3.ko",
		"cxgb4":        "cxgb4.ko",
		"cxgb4vf":      "cxgb4vf.ko",
		// "drbd":            "drbd.ko", // disabled, see https://github.com/siderolabs/pkgs/pull/873
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
		// "zfs":             "zfs.ko", // disabled, see https://github.com/siderolabs/pkgs/pull/873
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

	iscsiTargetExists := func() bool {
		var iscsiTargetExists bool

		resp, err := suite.Client.Disks(ctx)
		suite.Require().NoError(err)

		for _, msg := range resp.Messages {
			for _, disk := range msg.Disks {
				if disk.Modalias == "scsi:t-0x00" {
					iscsiTargetExists = true

					break
				}
			}
		}

		return iscsiTargetExists
	}

	if !iscsiTargetExists() {
		_, err := suite.Clientset.CoreV1().Pods("kube-system").Create(suite.ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "iscsi-test",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "iscsi-test",
						Image: "alpine",
						Command: []string{
							"tail",
							"-f",
							"/dev/null",
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: pointer.To(true),
						},
					},
				},
				HostNetwork: true,
				HostPID:     true,
			},
		}, metav1.CreateOptions{})
		defer suite.Clientset.CoreV1().Pods("kube-system").Delete(suite.ctx, "iscsi-test", metav1.DeleteOptions{}) //nolint:errcheck

		suite.Require().NoError(err)

		// wait for the pod to be ready
		suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "kube-system", "iscsi-test"))

		reader, err := suite.Client.Read(ctx, "/system/iscsi/initiatorname.iscsi")
		suite.Require().NoError(err)

		defer reader.Close() //nolint:errcheck

		body, err := io.ReadAll(reader)
		suite.Require().NoError(err)

		initiatorName := strings.TrimPrefix(strings.TrimSpace(string(body)), "InitiatorName=")

		stdout, stderr, err := suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"iscsi-test",
			fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- tgtadm --lld iscsi --op new --mode target --tid 1 -T %s", initiatorName),
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Equal("", stdout)

		stdout, stderr, err = suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"iscsi-test",
			"/bin/sh -c 'dd if=/dev/zero of=/proc/$(pgrep tgtd)/root/var/run/tgtd/iscsi.disk bs=1M count=100'",
		)
		suite.Require().NoError(err)

		suite.Require().Equal("100+0 records in\n100+0 records out\n", stderr)
		suite.Require().Equal("", stdout)

		stdout, stderr, err = suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"iscsi-test",
			"nsenter --mount=/proc/1/ns/mnt -- tgtadm --lld iscsi --op new --mode logicalunit --tid 1 --lun 1 -b /var/run/tgtd/iscsi.disk",
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Equal("", stdout)

		stdout, stderr, err = suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"iscsi-test",
			"nsenter --mount=/proc/1/ns/mnt -- tgtadm --lld iscsi --op bind --mode target --tid 1 -I ALL",
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Equal("", stdout)

		stdout, stderr, err = suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"iscsi-test",
			fmt.Sprintf("/bin/sh -c 'nsenter --mount=/proc/$(pgrep iscsid)/ns/mnt --net=/proc/$(pgrep iscsid)/ns/net -- iscsiadm --mode discovery --type sendtargets --portal %s:3260'", node),
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Equal(fmt.Sprintf("%s:3260,1 %s\n", node, initiatorName), stdout)

		stdout, stderr, err = suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"iscsi-test",
			fmt.Sprintf("/bin/sh -c 'nsenter --mount=/proc/$(pgrep iscsid)/ns/mnt --net=/proc/$(pgrep iscsid)/ns/net -- iscsiadm --mode node --targetname %s --portal %s:3260 --login'", initiatorName, node),
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Contains(stdout, "successful.")
	}

	suite.Eventually(func() bool {
		return iscsiTargetExists()
	}, 5*time.Second, 1*time.Second)
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
	_, err := suite.Clientset.NodeV1().RuntimeClasses().Create(suite.ctx, &nodev1.RuntimeClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "gvisor",
		},
		Handler: "runsc",
	}, metav1.CreateOptions{})
	defer suite.Clientset.NodeV1().RuntimeClasses().Delete(suite.ctx, "gvisor", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	_, err = suite.Clientset.CoreV1().Pods("default").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx-gvisor",
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: pointer.To("gvisor"),
			Containers: []corev1.Container{
				{
					Name:  "nginx-gvisor",
					Image: "nginx",
				},
			},
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("default").Delete(suite.ctx, "nginx-gvisor", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "default", "nginx-gvisor"))
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

	var mdADMArrayExists bool

	uuid := suite.mdADMScan()
	if uuid != "" {
		mdADMArrayExists = true
	}

	if !mdADMArrayExists {
		userDisks, err := suite.UserDisks(suite.ctx, node, 4)
		suite.Require().NoError(err)

		suite.Require().GreaterOrEqual(len(userDisks), 2, "expected at least two user disks with size greater than 4GB to be available")

		_, err = suite.Clientset.CoreV1().Pods("kube-system").Create(suite.ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "mdadm-create",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "mdadm-create",
						Image: "alpine",
						Command: []string{
							"tail",
							"-f",
							"/dev/null",
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: pointer.To(true),
						},
					},
				},
				HostNetwork: true,
				HostPID:     true,
			},
		}, metav1.CreateOptions{})
		defer suite.Clientset.CoreV1().Pods("kube-system").Delete(suite.ctx, "mdadm-create", metav1.DeleteOptions{}) //nolint:errcheck

		suite.Require().NoError(err)

		// wait for the pod to be ready
		suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "kube-system", "mdadm-create"))

		_, stderr, err := suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"mdadm-create",
			fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- mdadm --create --verbose /dev/md0 --metadata=0.90 --level=1 --raid-devices=2 %s", strings.Join(userDisks[:2], " ")),
		)
		suite.Require().NoError(err)

		suite.Require().Contains(stderr, "mdadm: array /dev/md0 started.")
	}

	// now we want to reboot the node and make sure the array is still mounted
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
	)

	suite.Require().NotEmpty(suite.mdADMScan())
}

func (suite *ExtensionsSuiteQEMU) mdADMScan() string {
	// create a random suffix for the mdadm-scan pod
	randomSuffix := make([]byte, 4)
	_, err := rand.Read(randomSuffix)
	suite.Require().NoError(err)

	podName := fmt.Sprintf("mdadm-scan-%x", randomSuffix)

	_, err = suite.Clientset.CoreV1().Pods("kube-system").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  podName,
					Image: "alpine",
					Command: []string{
						"tail",
						"-f",
						"/dev/null",
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.To(true),
					},
				},
			},
			HostNetwork: true,
			HostPID:     true,
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("kube-system").Delete(suite.ctx, podName, metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "kube-system", podName))

	stdout, stderr, err := suite.ExecuteCommandInPod(
		suite.ctx,
		"kube-system",
		podName,
		"nsenter --mount=/proc/1/ns/mnt -- mdadm --detail --scan",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)

	stdOutSplit := strings.Split(stdout, " ")

	return strings.TrimPrefix(stdOutSplit[len(stdOutSplit)-1], "UUID=")
}

// TestExtensionsZFS verifies zfs is working, udev rules work and the pool is mounted on reboot.
func (suite *ExtensionsSuiteQEMU) TestExtensionsZFS() {
	suite.T().Skip("skipping due to https://github.com/siderolabs/pkgs/pull/873")

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.AssertServicesRunning(suite.ctx, node, map[string]string{"ext-zpool-importer": "Finished"})

	ctx := client.WithNode(suite.ctx, node)

	var zfsPoolExists bool

	_, err := suite.Clientset.CoreV1().Pods("kube-system").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "zpool-list",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "zpool-list",
					Image: "alpine",
					Command: []string{
						"tail",
						"-f",
						"/dev/null",
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.To(true),
					},
				},
			},
			HostNetwork: true,
			HostPID:     true,
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("kube-system").Delete(suite.ctx, "zpool-list", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "kube-system", "zpool-list"))

	stdout, stderr, err := suite.ExecuteCommandInPod(
		suite.ctx,
		"kube-system",
		"zpool-list",
		"nsenter --mount=/proc/1/ns/mnt -- zpool list",
	)
	suite.Require().NoError(err)

	suite.Require().Equal("", stderr)
	suite.Require().NotEmpty(stdout)

	if stdout != "no pools available\n" {
		zfsPoolExists = true
	}

	if !zfsPoolExists {
		userDisks, err := suite.UserDisks(suite.ctx, node, 4)
		suite.Require().NoError(err)

		suite.Require().NotEmpty(userDisks, "expected at least one user disk with size greater than 4GB to be available")

		_, err = suite.Clientset.CoreV1().Pods("kube-system").Create(suite.ctx, &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "zpool-create",
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "zpool-create",
						Image: "alpine",
						Command: []string{
							"tail",
							"-f",
							"/dev/null",
						},
						SecurityContext: &corev1.SecurityContext{
							Privileged: pointer.To(true),
						},
					},
				},
				HostNetwork: true,
				HostPID:     true,
			},
		}, metav1.CreateOptions{})
		defer suite.Clientset.CoreV1().Pods("kube-system").Delete(suite.ctx, "zpool-create", metav1.DeleteOptions{}) //nolint:errcheck

		suite.Require().NoError(err)

		// wait for the pod to be ready
		suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 5*time.Minute, "kube-system", "zpool-create"))

		stdout, stderr, err := suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"zpool-create",
			fmt.Sprintf("nsenter --mount=/proc/1/ns/mnt -- zpool create -m /var/tank tank %s", userDisks[0]),
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Equal("", stdout)

		stdout, stderr, err = suite.ExecuteCommandInPod(
			suite.ctx,
			"kube-system",
			"zpool-create",
			"nsenter --mount=/proc/1/ns/mnt -- zfs create -V 1gb tank/vol",
		)
		suite.Require().NoError(err)

		suite.Require().Equal("", stderr)
		suite.Require().Equal("", stdout)
	}

	checkZFSPoolMounted := func() bool {
		mountsResp, err := suite.Client.Mounts(ctx)
		suite.Require().NoError(err)

		for _, msg := range mountsResp.Messages {
			for _, stats := range msg.Stats {
				if stats.MountedOn == "/var/tank" {
					return true
				}
			}
		}

		return false
	}

	checkZFSVolumePathPopulatedByUdev := func() {
		// this is the path that udev will populate, which is a symlink to the actual device
		path := "/dev/zvol/tank/vol"

		stream, err := suite.Client.LS(ctx, &machineapi.ListRequest{
			Root: path,
		})

		suite.Require().NoError(err)

		suite.Require().NoError(helpers.ReadGRPCStream(stream, func(info *machineapi.FileInfo, node string, multipleNodes bool) error {
			suite.Require().Equal("/dev/zd0", info.Name, "expected %s to exist", path)

			return nil
		}))
	}

	suite.Require().True(checkZFSPoolMounted())
	checkZFSVolumePathPopulatedByUdev()

	// now we want to reboot the node and make sure the pool is still mounted
	suite.AssertRebooted(
		suite.ctx, node, func(nodeCtx context.Context) error {
			return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
		}, 5*time.Minute,
	)

	suite.Require().True(checkZFSPoolMounted())
	checkZFSVolumePathPopulatedByUdev()
}

// TestExtensionsUtilLinuxTools verifies util-linux-tools are working.
func (suite *ExtensionsSuiteQEMU) TestExtensionsUtilLinuxTools() {
	_, err := suite.Clientset.CoreV1().Pods("kube-system").Create(suite.ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "util-linux-tools-test",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "util-linux-tools-test",
					Image: "alpine",
					Command: []string{
						"tail",
						"-f",
						"/dev/null",
					},
					SecurityContext: &corev1.SecurityContext{
						Privileged: pointer.To(true),
					},
				},
			},
			HostNetwork: true,
			HostPID:     true,
		},
	}, metav1.CreateOptions{})
	defer suite.Clientset.CoreV1().Pods("kube-system").Delete(suite.ctx, "util-linux-tools-test", metav1.DeleteOptions{}) //nolint:errcheck

	suite.Require().NoError(err)

	// wait for the pod to be ready
	suite.Require().NoError(suite.WaitForPodToBeRunning(suite.ctx, 10*time.Minute, "kube-system", "util-linux-tools-test"))

	stdout, stderr, err := suite.ExecuteCommandInPod(
		suite.ctx,
		"kube-system",
		"util-linux-tools-test",
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

func init() {
	allSuites = append(allSuites, &ExtensionsSuiteQEMU{})
}
