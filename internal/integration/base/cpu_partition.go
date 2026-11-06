// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package base

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"k8s.io/utils/cpuset"

	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// CPUPartitionRebootWarning is the warning apply returns for a CPU partition policy differing from the one the node booted with.
const CPUPartitionRebootWarning = "CPUPartitionConfig: the CPU partition policy changes take effect only after a reboot"

// cpuPartitionRebootTimeout bounds one reboot of a node applying or dropping a CPU partition policy.
const cpuPartitionRebootTimeout = 10 * time.Minute

// ReadOnlineCPUs returns the online CPUs of the node.
func (apiSuite *APISuite) ReadOnlineCPUs(nodeCtx context.Context) cpuset.CPUSet {
	online, err := cpuset.Parse(apiSuite.ReadFile(nodeCtx, "/sys/devices/system/cpu/online"))
	apiSuite.Require().NoError(err)

	return online
}

// AssertCgroupCPUs waits until a CPU list file of a cgroup holds exactly the CPUs, compared as sets.
func (apiSuite *APISuite) AssertCgroupCPUs(nodeCtx context.Context, root, file string, expected cpuset.CPUSet) {
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

// RequireNoCPUPartitionConfig skips the test if the node already declares a CPU partition policy
// and returns the exact bytes of its current configuration.
func (apiSuite *APISuite) RequireNoCPUPartitionConfig(nodeCtx context.Context, node string) []byte {
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

// ApplyConfigWarnings applies the patched configuration without a reboot and returns the warnings of the response.
func (apiSuite *APISuite) ApplyConfigWarnings(nodeCtx context.Context, patch func(config.Provider) (config.Provider, error)) []string {
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

// RebootForCPUPartition reboots the node, so the CPU partition policy of the active configuration
// becomes the policy of the boot. Cluster health is only checked where Kubernetes runs.
func (apiSuite *APISuite) RebootForCPUPartition(ctx context.Context, node string) {
	reboot := func(nodeCtx context.Context) error {
		return IgnoreGRPCUnavailable(apiSuite.Client.Reboot(nodeCtx))
	}

	if apiSuite.Cluster != nil && apiSuite.Capabilities().SupportsKubernetes {
		apiSuite.AssertRebooted(ctx, node, reboot, cpuPartitionRebootTimeout)

		return
	}

	apiSuite.AssertRebootedNoChecks(ctx, node, reboot, cpuPartitionRebootTimeout)
	apiSuite.WaitForBootDone(ctx)
}

// RestoreCPUPartitionOnCleanup registers restoring the exact original configuration and rebooting
// the node, which is the only way a CPU partition policy of the boot goes away.
//
// It must be registered before the first apply, and uses a fresh context: the test deadline may
// have expired by then.
func (apiSuite *APISuite) RestoreCPUPartitionOnCleanup(node string, originalBytes []byte) {
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

		apiSuite.RebootForCPUPartition(cleanupCtx, node)

		rtestutils.AssertResource(nodeCtx, apiSuite.T(), apiSuite.Client.State(), runtime.CPUPartitionSpecID,
			func(spec *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
				asrt.False(spec.TypedSpec().Enabled, "node %s still applies a CPU partition policy", node)
			})
	})
}

// AssertCPUPartitionReady waits until the CPU partition policy of the boot is applied.
func (apiSuite *APISuite) AssertCPUPartitionReady(nodeCtx context.Context) *runtime.CPUPartitionStatusSpec {
	apiSuite.T().Helper()

	var status runtime.CPUPartitionStatusSpec

	rtestutils.AssertResource(nodeCtx, apiSuite.T(), apiSuite.Client.State(), runtime.CPUPartitionStatusID,
		func(res *runtime.CPUPartitionStatus, asrt *assert.Assertions) {
			asrt.True(res.TypedSpec().Ready, "CPU partition policy of the boot is not applied, see machined logs")

			status = *res.TypedSpec()
		})

	return &status
}
