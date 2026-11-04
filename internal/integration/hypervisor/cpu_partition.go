// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package hypervisor

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"go.yaml.in/yaml/v4"
	"k8s.io/utils/cpuset"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// cpuPartitionBootMinCPUs is the smallest node the test can split: one CPU for the host roots, the
// rest for kubepods, and a further CPU taken away from kubepods by the edit pending a reboot.
const cpuPartitionBootMinCPUs = 3

// CPUPartitionSuite verifies the CPU partition policy of the boot on a real node.
type CPUPartitionSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *CPUPartitionSuite) SuiteName() string {
	return "hypervisor.CPUPartitionSuite"
}

// SetupTest ...
func (suite *CPUPartitionSuite) SetupTest() {
	if testing.Short() {
		suite.T().Skip("skipping reboots in short mode")
	}

	if !suite.Capabilities().SupportsReboot {
		suite.T().Skip("cluster doesn't support reboots")
	}

	// Two reboots, each up to 10 minutes, plus the assertions between them.
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 30*time.Minute)
}

// TearDownTest ...
func (suite *CPUPartitionSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestBootPolicyLifecycle applies a policy, reboots to make it the policy of the boot, and checks it
// is applied; edits and removal are accepted with a warning and change nothing until a reboot.
func (suite *CPUPartitionSuite) TestBootPolicyLifecycle() {
	// Control planes are the larger nodes in the default e2e clusters; workers often have too few CPUs.
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)
	nodeCtx := client.WithNode(suite.ctx, node)

	online := readOnlineCPUs(nodeCtx, &suite.APISuite)
	if online.Size() < cpuPartitionBootMinCPUs {
		suite.T().Skipf("node %s has %d online CPUs, the test needs at least %d", node, online.Size(), cpuPartitionBootMinCPUs)
	}

	cpus := online.List()
	host := cpuset.New(cpus[0])
	kubepods := cpuset.New(cpus[1:]...)
	reserved := online.Difference(kubepods)

	originalBytes := requireNoCPUPartitionConfig(nodeCtx, &suite.APISuite, node)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), runtime.CPUPartitionSpecID,
		func(spec *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
			asrt.Empty(spec.TypedSpec().Roots, "node %s booted with a CPU partition policy", node)
		})

	restoreCPUPartitionOnCleanup(&suite.APISuite, node, originalBytes)

	suite.T().Logf("node %s online CPUs %q: host roots on %q, kubepods on %q", node, online, host, kubepods)

	// The policy is not live before the reboot: apply only warns.
	suite.Require().Contains(suite.applyCPUPartition(nodeCtx, cpuPartitionDocument(host, kubepods)), cpuPartitionRebootWarning)

	rebootForCPUPartition(suite.ctx, &suite.APISuite, node)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), runtime.CPUPartitionSpecID,
		func(spec *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
			asrt.Equal(map[string]string{
				constants.CgroupInit:           host.String(),
				constants.CgroupSystem:         host.String(),
				constants.CgroupPodRuntimeRoot: host.String(),
				constants.CgroupKubepods:       kubepods.String(),
			}, spec.TypedSpec().Roots)
		})

	status := assertCPUPartitionReady(nodeCtx, &suite.APISuite)
	suite.Require().Equal(reserved.String(), status.KubeletReservedCPUs)
	suite.assertRootCPUs(nodeCtx, host, kubepods)
	assertKubeletCPUManager(suite.ctx, &suite.APISuite, node, kubepods, reserved)

	// An edit is not applied live.
	suite.Require().Contains(suite.applyCPUPartition(nodeCtx, cpuPartitionDocument(host, cpuset.New(cpus[2:]...))), cpuPartitionRebootWarning)
	suite.assertRootCPUs(nodeCtx, host, kubepods)

	// Neither is a removal.
	suite.Require().Contains(suite.applyCPUPartition(nodeCtx, nil), cpuPartitionRebootWarning)
	suite.assertRootCPUs(nodeCtx, host, kubepods)

	// Going back to the policy of the boot warns no more.
	suite.Require().NotContains(suite.applyCPUPartition(nodeCtx, cpuPartitionDocument(host, kubepods)), cpuPartitionRebootWarning)
	assertCPUPartitionReady(nodeCtx, &suite.APISuite)
}

// applyCPUPartition replaces the CPU partition document, removing it for nil, and returns the warnings of the apply.
func (suite *CPUPartitionSuite) applyCPUPartition(nodeCtx context.Context, doc *runtimecfg.CPUPartitionConfigV1Alpha1) []string {
	return applyConfigWarnings(nodeCtx, &suite.APISuite, func(cfg config.Provider) (config.Provider, error) {
		docs := xslices.Filter(cfg.Documents(), func(d configconfig.Document) bool { return d.Kind() != runtimecfg.CPUPartitionConfigKind })

		if doc != nil {
			docs = append(docs, doc)
		}

		return container.New(docs...)
	})
}

func cpuPartitionDocument(host, kubepods cpuset.CPUSet) *runtimecfg.CPUPartitionConfigV1Alpha1 {
	doc := runtimecfg.NewCPUPartitionConfigV1Alpha1()
	doc.InitConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: host.String()}
	doc.SystemConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: host.String()}
	doc.PodRuntimeConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: host.String()}
	doc.KubepodsConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: kubepods.String()}

	return doc
}

// assertRootCPUs checks both the configured and the effective CPUs of every bounded root.
func (suite *CPUPartitionSuite) assertRootCPUs(nodeCtx context.Context, host, kubepods cpuset.CPUSet) {
	for root, cpus := range map[string]cpuset.CPUSet{
		constants.CgroupInit:           host,
		constants.CgroupSystem:         host,
		constants.CgroupPodRuntimeRoot: host,
		constants.CgroupKubepods:       kubepods,
	} {
		assertCgroupCPUs(nodeCtx, &suite.APISuite, root, "cpuset.cpus", cpus)
		assertCgroupCPUs(nodeCtx, &suite.APISuite, root, "cpuset.cpus.effective", cpus)
	}
}

// assertKubeletCPUManager checks the kubelet runs the static CPU manager policy, and that a kubelet
// restart leaves the kubepods mask at its bound. It does nothing where Kubernetes does not run.
func assertKubeletCPUManager(ctx context.Context, apiSuite *base.APISuite, node string, kubepods, reserved cpuset.CPUSet) {
	if !apiSuite.Capabilities().SupportsKubernetes {
		return
	}

	nodeCtx := client.WithNode(ctx, node)

	apiSuite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		reader, err := apiSuite.Client.Read(nodeCtx, "/etc/kubernetes/kubelet.yaml")
		if !assert.NoError(collect, err) {
			return
		}

		var rendered struct {
			CPUManagerPolicy   string `yaml:"cpuManagerPolicy"`
			ReservedSystemCPUs string `yaml:"reservedSystemCPUs"`
		}

		err = yaml.NewDecoder(reader).Decode(&rendered)
		closeErr := reader.Close()

		if !assert.NoError(collect, err) || !assert.NoError(collect, closeErr) {
			return
		}

		assert.Equal(collect, "static", rendered.CPUManagerPolicy)

		actual, err := cpuset.Parse(rendered.ReservedSystemCPUs)
		if assert.NoError(collect, err) {
			assert.True(collect, actual.Equals(reserved), "kubelet reservedSystemCPUs %q, expected %q", actual, reserved)
		}
	}, time.Minute, time.Second, "the kubelet should run the static CPU manager policy reserving %q", reserved)

	restartSince := apiSuite.LatestServiceEventTimestamp(ctx, node, "kubelet")

	_, err := apiSuite.Client.ServiceRestart(nodeCtx, "kubelet")
	apiSuite.Require().NoError(err)

	apiSuite.AssertServiceEventsInOrder(ctx, node, "kubelet", restartSince, []string{"Stopping", "Finished", "Starting", "Waiting", "Preparing", "Running"})

	rtestutils.AssertResource(nodeCtx, apiSuite.T(), apiSuite.Client.State(), "kubelet",
		func(svc *v1alpha1.Service, asrt *assert.Assertions) {
			asrt.True(svc.TypedSpec().Healthy)
		})

	assertCgroupCPUs(nodeCtx, apiSuite, constants.CgroupKubepods, "cpuset.cpus", kubepods)
}

func init() {
	allSuites = append(allSuites, new(CPUPartitionSuite))
}

// cpuPartitionRebootWarning is the warning apply returns for a CPU partition policy differing from the one the node booted with.
const cpuPartitionRebootWarning = "CPUPartitionConfig: the CPU partition policy changes take effect only after a reboot"

// cpuPartitionRebootTimeout bounds one reboot of a node applying or dropping a CPU partition policy.
const cpuPartitionRebootTimeout = 10 * time.Minute

// readOnlineCPUs returns the online CPUs of the node.
func readOnlineCPUs(nodeCtx context.Context, apiSuite *base.APISuite) cpuset.CPUSet {
	online, err := cpuset.Parse(apiSuite.ReadFile(nodeCtx, "/sys/devices/system/cpu/online"))
	apiSuite.Require().NoError(err)

	return online
}

// assertCgroupCPUs waits until a CPU list file of a cgroup holds exactly the CPUs, compared as sets.
func assertCgroupCPUs(nodeCtx context.Context, apiSuite *base.APISuite, root, file string, expected cpuset.CPUSet) {
	apiSuite.T().Helper()

	apiSuite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		contents, err := apiSuite.ReadCgroupFile(nodeCtx, root, file)
		if !assert.NoError(collect, err) {
			return
		}

		actual, err := cpuset.Parse(contents)
		if assert.NoError(collect, err, "%s/%s: %q", root, file, contents) {
			assert.True(collect, actual.Equals(expected), "%s/%s is %q, expected %q", root, file, actual, expected)
		}
	}, time.Minute, time.Second, "%s/%s should hold CPUs %q", root, file, expected)
}

// requireNoCPUPartitionConfig skips the test if the node already declares a CPU partition policy
// and returns the exact bytes of its current configuration.
func requireNoCPUPartitionConfig(nodeCtx context.Context, apiSuite *base.APISuite, node string) []byte {
	apiSuite.T().Helper()

	original, err := apiSuite.ReadConfigFromNode(nodeCtx)
	apiSuite.Require().NoError(err)

	if original.CPUPartitionConfig() != nil {
		apiSuite.T().Skipf("node %s already carries a CPUPartitionConfig document", node)
	}

	originalBytes, err := original.Bytes()
	apiSuite.Require().NoError(err)

	return originalBytes
}

// applyConfigWarnings applies the patched configuration without a reboot and returns the warnings of the response.
func applyConfigWarnings(nodeCtx context.Context, apiSuite *base.APISuite, patch func(config.Provider) (config.Provider, error)) []string {
	cfg, err := apiSuite.ReadConfigFromNode(nodeCtx)
	apiSuite.Require().NoError(err)

	patched, err := patch(cfg)
	apiSuite.Require().NoError(err)

	data, err := patched.Bytes()
	apiSuite.Require().NoError(err)

	resp, err := apiSuite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
		Data: data,
		Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
	})
	apiSuite.Require().NoError(err)
	apiSuite.Require().Len(resp.GetMessages(), 1)

	return resp.GetMessages()[0].GetWarnings()
}

// rebootForCPUPartition reboots the node, so the CPU partition policy of the active configuration
// becomes the policy of the boot. Cluster health is only checked where Kubernetes runs.
func rebootForCPUPartition(ctx context.Context, apiSuite *base.APISuite, node string) {
	reboot := func(nodeCtx context.Context) error {
		return base.IgnoreGRPCUnavailable(apiSuite.Client.Reboot(nodeCtx))
	}

	if apiSuite.Cluster != nil && apiSuite.Capabilities().SupportsKubernetes {
		apiSuite.AssertRebooted(ctx, node, reboot, cpuPartitionRebootTimeout)

		return
	}

	apiSuite.AssertRebootedNoChecks(ctx, node, reboot, cpuPartitionRebootTimeout)
	apiSuite.WaitForBootDone(ctx)
}

// restoreCPUPartitionOnCleanup registers restoring the exact original configuration and rebooting
// the node, which is the only way a CPU partition policy of the boot goes away.
//
// It must be registered before the first apply, and uses a fresh context: the test deadline may
// have expired by then.
func restoreCPUPartitionOnCleanup(apiSuite *base.APISuite, node string, originalBytes []byte) {
	apiSuite.T().Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 2*cpuPartitionRebootTimeout)
		defer cleanupCancel()

		nodeCtx := client.WithNode(cleanupCtx, node)

		_, applyErr := apiSuite.Client.ApplyConfiguration(nodeCtx, &machineapi.ApplyConfigurationRequest{
			Data: originalBytes,
			Mode: machineapi.ApplyConfigurationRequest_NO_REBOOT,
		})
		if !apiSuite.Assert().NoError(applyErr, "restore original configuration on node %s", node) {
			return
		}

		rebootForCPUPartition(cleanupCtx, apiSuite, node)

		rtestutils.AssertResource(nodeCtx, apiSuite.T(), apiSuite.Client.State(), runtime.CPUPartitionSpecID,
			func(spec *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
				asrt.Empty(spec.TypedSpec().Roots, "node %s still applies a CPU partition policy", node)
			})
	})
}

// assertCPUPartitionReady waits until the CPU partition policy of the boot is applied.
func assertCPUPartitionReady(nodeCtx context.Context, apiSuite *base.APISuite) *runtime.CPUPartitionStatusSpec {
	apiSuite.T().Helper()

	var status runtime.CPUPartitionStatusSpec

	rtestutils.AssertResource(nodeCtx, apiSuite.T(), apiSuite.Client.State(), runtime.CPUPartitionStatusID,
		func(res *runtime.CPUPartitionStatus, asrt *assert.Assertions) {
			asrt.True(res.TypedSpec().Ready, "CPU partition policy of the boot is not applied: %s", res.TypedSpec().Error)

			status = *res.TypedSpec()
		})

	return &status
}
