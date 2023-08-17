// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// ExtensionsSuite verifies Talos is securebooted.
type ExtensionsSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// ExtensionsTestType specifies the type of extensions test to run.
type ExtensionsTestType string

const (
	// ExtensionsTestTypeNone disables extensions tests.
	ExtensionsTestTypeNone ExtensionsTestType = "none"
	// ExtensionsTestTypeQEMU enables qemu extensions tests.
	ExtensionsTestTypeQEMU ExtensionsTestType = "qemu"
	// ExtensionsTestTypeNvidia enables nvidia extensions tests.
	ExtensionsTestTypeNvidia ExtensionsTestType = "nvidia"
	// ExtensionsTestTypeNvidiaFabricManager enables nvidia fabric manager extensions tests.
	ExtensionsTestTypeNvidiaFabricManager ExtensionsTestType = "nvidia-fabricmanager"
)

// SuiteName ...
func (suite *ExtensionsSuite) SuiteName() string {
	return "api.ExtensionsSuite"
}

// SetupTest ...
func (suite *ExtensionsSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping in short mode")
	}

	if suite.Cluster.Provisioner() == provisionerDocker {
		suite.T().Skip("skipping extensions tests in docker")
	}

	if suite.ExtensionsTestType == string(ExtensionsTestTypeNone) {
		suite.T().Skip("skipping as extensions test are not enabled")
	}

	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *ExtensionsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestExtensionsExpectedPaths verifies expected paths are present.
func (suite *ExtensionsSuite) TestExtensionsExpectedPaths() {
	if suite.ExtensionsTestType != string(ExtensionsTestTypeQEMU) {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

	expectedPaths := []string{
		"/lib/firmware/amd-ucode",
		"/lib/firmware/bnx2x",
		"/lib/firmware/i915",
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
func (suite *ExtensionsSuite) TestExtensionsExpectedModules() {
	// expectedModulesModDep is a map of module name to module.dep name
	expectedModulesModDep := map[string]string{
		"asix":            "asix.ko",
		"ax88179_178a":    "ax88179_178a.ko",
		"ax88796b":        "ax88796b.ko",
		"btrfs":           "btrfs.ko",
		"cdc_ether":       "cdc_ether.ko",
		"cdc_mbim":        "cdc_mbim.ko",
		"cdc_ncm":         "cdc_ncm.ko",
		"cdc_subset":      "cdc_subset.ko",
		"cdc_wdm":         "cdc-wdm.ko",
		"drbd":            "drbd.ko",
		"gasket":          "gasket.ko",
		"net1080":         "net1080.ko",
		"option":          "option.ko",
		"qmi_wwan":        "qmi_wwan.ko",
		"r8153_ecm":       "r8153_ecm.ko",
		"thunderbolt":     "thunderbolt.ko",
		"thunderbolt_net": "thunderbolt-net.ko",
		"usb_wwan":        "usb_wwan.ko",
		"usbnet":          "usbnet.ko",
		"zaurus":          "zaurus.ko",
		"zfs":             "zfs.ko",
	}

	if suite.ExtensionsTestType == string(ExtensionsTestTypeNvidia) || suite.ExtensionsTestType == string(ExtensionsTestTypeNvidiaFabricManager) {
		expectedModulesModDep = map[string]string{
			"nvidia": "nvidia.ko",
		}
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	fileReader, err := suite.Client.Read(ctx, "/proc/modules")
	defer func() {
		err = fileReader.Close()
	}()

	suite.Require().NoError(err)

	scanner := bufio.NewScanner(fileReader)

	var loadedModules []string

	for scanner.Scan() {
		loadedModules = append(loadedModules, strings.Split(scanner.Text(), " ")[0])
	}
	suite.Require().NoError(scanner.Err())

	fileReader, err = suite.Client.Read(ctx, fmt.Sprintf("/lib/modules/%s/modules.dep", constants.DefaultKernelVersion))
	defer func() {
		err = fileReader.Close()
	}()

	suite.Require().NoError(err)

	scanner = bufio.NewScanner(fileReader)

	var modulesDep []string

	for scanner.Scan() {
		modulesDep = append(modulesDep, filepath.Base(strings.Split(scanner.Text(), ":")[0]))
	}
	suite.Require().NoError(scanner.Err())

	for module, moduleDep := range expectedModulesModDep {
		suite.Require().Contains(loadedModules, module, "expected %s to be loaded", module)
		suite.Require().Contains(modulesDep, moduleDep, "expected %s to be in modules.dep", moduleDep)
	}
}

// TestExtensionsISCSI verifies expected services are running.
func (suite *ExtensionsSuite) TestExtensionsISCSI() {
	expectedServices := map[string]string{
		"ext-iscsid": "Running",
		"ext-tgtd":   "Running",
	}

	suite.testServicesRunning(expectedServices)
}

// TestExtensionsNutClient verifies nut client is working.
func (suite *ExtensionsSuite) TestExtensionsNutClient() {
	if suite.ExtensionsTestType != string(ExtensionsTestTypeQEMU) {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

	suite.testServicesRunning(map[string]string{"ext-nut-client": "Running"})
}

// TestExtensionsQEMUGuestAgent verifies qemu guest agent is working.
func (suite *ExtensionsSuite) TestExtensionsQEMUGuestAgent() {
	if suite.ExtensionsTestType != string(ExtensionsTestTypeQEMU) || suite.Cluster.Provisioner() != "qemu" {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

	suite.testServicesRunning(map[string]string{"ext-qemu-guest-agent": "Running"})

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
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
func (suite *ExtensionsSuite) TestExtensionsTailscale() {
	if suite.ExtensionsTestType != string(ExtensionsTestTypeQEMU) {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

	// Tailscale service keeps on restarting unless authed, so this test is disabled for now.
	if ok := os.Getenv("TALOS_INTEGRATION_RUN_TAILSCALE"); ok == "" {
		suite.T().Skip("skipping as tailscale integration tests are not enabled")
	}

	suite.testServicesRunning(map[string]string{"ext-tailscale": "Running"})

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
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
func (suite *ExtensionsSuite) TestExtensionsHelloWorldService() {
	if suite.ExtensionsTestType != string(ExtensionsTestTypeQEMU) {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)

	suite.testServicesRunning(map[string]string{
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
func (suite *ExtensionsSuite) TestExtensionsGvisor() {
	if suite.ExtensionsTestType != string(ExtensionsTestTypeQEMU) {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

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

// TestExtensionsZFS verifies zfs is working, udev rules work and the pool is mounted on reboot.
func (suite *ExtensionsSuite) TestExtensionsZFS() {
	if suite.ExtensionsTestType != string(ExtensionsTestTypeQEMU) {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

	suite.testServicesRunning(map[string]string{"ext-zpool-importer": "Finished"})

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	var zfsPoolExists bool

	userDisks, err := suite.UserDisks(suite.ctx, node, 4)
	suite.Require().NoError(err)

	suite.Require().NotEmpty(userDisks, "expected at least one user disk with size greater than 4GB to be available")

	resp, err := suite.Client.LS(ctx, &machineapi.ListRequest{
		Root: fmt.Sprintf("/dev/%s1", userDisks[0]),
	})
	suite.Require().NoError(err)

	if _, err = resp.Recv(); err == nil {
		zfsPoolExists = true
	}

	if !zfsPoolExists {
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
							Privileged: pointer.Bool(true),
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

func (suite *ExtensionsSuite) testServicesRunning(serviceStatus map[string]string) {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	for svc, state := range serviceStatus {
		resp, err := suite.Client.ServiceInfo(ctx, svc)
		suite.Require().NoError(err)
		suite.Require().NotNil(resp, "expected service %s to be registered", svc)

		for _, svcInfo := range resp {
			suite.Require().Equal(state, svcInfo.Service.State, "expected service %s to have state %s", svc, state)
		}
	}
}

func init() {
	allSuites = append(allSuites, &ExtensionsSuite{})
}
