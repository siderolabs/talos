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
	"github.com/siderolabs/go-retry/retry"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/helpers"
	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
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

// TestExtensionsExpectedServices verifies expected services are running.
func (suite *ExtensionsSuite) TestExtensionsExpectedServices() {
	expectedServices := []string{
		"ext-hello-world",
		"ext-iscsid",
		"ext-nut-client",
		"ext-qemu-guest-agent",
		"ext-tgtd",
	}

	// Tailscale service keeps on restarting unless authed, so this test is disabled for now.
	if ok := os.Getenv("TALOS_INTEGRATION_RUN_TAILSCALE"); ok != "" {
		expectedServices = append(expectedServices, "ext-tailscale")
	}

	switch ExtensionsTestType(suite.ExtensionsTestType) {
	case ExtensionsTestTypeNone:
	case ExtensionsTestTypeQEMU:
	case ExtensionsTestTypeNvidia:
		expectedServices = []string{"ext-nvidia-persistenced"}
	case ExtensionsTestTypeNvidiaFabricManager:
		expectedServices = []string{
			"ext-nvidia-persistenced",
			"ext-nvidia-fabricmanager",
		}
	}

	suite.testServicesRunning(expectedServices)
}

// TestExtensionsQEMUGuestAgent verifies qemu guest agent is working.
func (suite *ExtensionsSuite) TestExtensionsQEMUGuestAgent() {
	if suite.ExtensionsTestType != string(ExtensionsTestTypeQEMU) || suite.Cluster.Provisioner() != "qemu" {
		suite.T().Skip("skipping as qemu extensions test are not enabled")
	}

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	hostnameSpec, err := safe.StateWatchFor[*network.HostnameStatus](
		ctx,
		suite.Client.COSI,
		network.NewHostnameStatus(network.NamespaceName, resource.ID("hostname")).Metadata(),
		state.WithEventTypes(state.Created, state.Updated),
	)
	suite.Require().NoError(err)

	bootID, err := suite.ReadBootID(ctx)
	suite.Require().NoError(err)

	clusterStatePath, err := suite.Cluster.StatePath()
	suite.Require().NoError(err)

	conn, err := net.Dial("unix", filepath.Join(clusterStatePath, hostnameSpec.TypedSpec().Hostname+".sock"))
	suite.Require().NoError(err)

	defer conn.Close() //nolint:errcheck

	_, err = conn.Write([]byte(`{"execute":"guest-shutdown", "arguments": {"mode": "reboot"}}`))
	suite.Require().NoError(err)

	suite.AssertBootIDChanged(ctx, bootID, node, time.Minute*5)
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
	suite.Require().NoError(retry.Constant(4*time.Minute, retry.WithUnits(time.Second*10)).Retry(
		func() error {
			pod, err := suite.Clientset.CoreV1().Pods("default").Get(suite.ctx, "nginx-gvisor", metav1.GetOptions{})
			if err != nil {
				return retry.ExpectedErrorf("error getting pod: %s", err)
			}

			if pod.Status.Phase != corev1.PodRunning {
				return retry.ExpectedErrorf("pod is not running yet: %s", pod.Status.Phase)
			}

			return nil
		},
	))
}

func (suite *ExtensionsSuite) testServicesRunning(services []string) {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	ctx := client.WithNode(suite.ctx, node)

	items, err := safe.StateListAll[*v1alpha1.Service](ctx, suite.Client.COSI)
	suite.Require().NoError(err)

	for _, expected := range services {
		svc, found := items.Find(func(s *v1alpha1.Service) bool {
			return s.Metadata().ID() == expected
		})
		if !found {
			suite.T().Fatalf("expected %s to be registered", expected)
		}

		suite.Require().True(svc.TypedSpec().Running, "expected %s to be running", expected)
	}
}

func init() {
	allSuites = append(allSuites, &ExtensionsSuite{})
}
