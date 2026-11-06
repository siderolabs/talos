// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

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
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// cpuPartitionMinCPUs is the smallest node the test can split: one CPU for the host roots, the
// rest for kubepods, and a further CPU taken away from kubepods by the edit pending a reboot.
const cpuPartitionMinCPUs = 3

// CPUPartitionSuite verifies the CPU partition policy of the boot on a real node.
type CPUPartitionSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *CPUPartitionSuite) SuiteName() string {
	return "api.CPUPartitionSuite"
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
	node := suite.RandomDiscoveredNodeInternalIP()
	nodeCtx := client.WithNode(suite.ctx, node)

	online := suite.ReadOnlineCPUs(nodeCtx)
	if online.Size() < cpuPartitionMinCPUs {
		suite.T().Skipf("node %s has %d online CPUs, the test needs at least %d", node, online.Size(), cpuPartitionMinCPUs)
	}

	cpus := online.List()
	host := cpuset.New(cpus[0])
	kubepods := cpuset.New(cpus[1:]...)
	reserved := online.Difference(kubepods)

	originalBytes := suite.RequireNoCPUPartitionConfig(nodeCtx, node)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), runtime.CPUPartitionSpecID,
		func(spec *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
			asrt.False(spec.TypedSpec().Enabled, "node %s booted with a CPU partition policy", node)
		})

	suite.RestoreCPUPartitionOnCleanup(node, originalBytes)

	suite.T().Logf("node %s online CPUs %q: host roots on %q, kubepods on %q", node, online, host, kubepods)

	// The policy is not live before the reboot: apply only warns.
	suite.Require().Contains(suite.applyCPUPartition(nodeCtx, cpuPartitionDocument(host, kubepods)), base.CPUPartitionRebootWarning)

	suite.RebootForCPUPartition(suite.ctx, node)

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), runtime.CPUPartitionSpecID,
		func(spec *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
			asrt.True(spec.TypedSpec().Enabled)
			asrt.Equal(map[string]string{
				constants.CgroupInit:           host.String(),
				constants.CgroupSystem:         host.String(),
				constants.CgroupPodRuntimeRoot: host.String(),
				constants.CgroupKubepods:       kubepods.String(),
			}, spec.TypedSpec().Roots)
		})

	status := suite.AssertCPUPartitionReady(nodeCtx)
	suite.Require().Equal(reserved.String(), status.KubeletReservedCPUs)
	suite.assertRootCPUs(nodeCtx, host, kubepods)
	suite.assertKubelet(nodeCtx, node, kubepods, reserved)

	// An edit is not applied live.
	suite.Require().Contains(suite.applyCPUPartition(nodeCtx, cpuPartitionDocument(host, cpuset.New(cpus[2:]...))), base.CPUPartitionRebootWarning)
	suite.assertRootCPUs(nodeCtx, host, kubepods)

	// Neither is a removal.
	suite.Require().Contains(suite.applyCPUPartition(nodeCtx, nil), base.CPUPartitionRebootWarning)
	suite.assertRootCPUs(nodeCtx, host, kubepods)

	// Going back to the policy of the boot warns no more.
	suite.Require().NotContains(suite.applyCPUPartition(nodeCtx, cpuPartitionDocument(host, kubepods)), base.CPUPartitionRebootWarning)
	suite.AssertCPUPartitionReady(nodeCtx)
}

// applyCPUPartition replaces the CPU partition document, removing it for nil, and returns the warnings of the apply.
func (suite *CPUPartitionSuite) applyCPUPartition(nodeCtx context.Context, doc *runtimecfg.CPUPartitionConfigV1Alpha1) []string {
	return suite.ApplyConfigWarnings(nodeCtx, func(cfg config.Provider) (config.Provider, error) {
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
		suite.AssertCgroupCPUs(nodeCtx, root, "cpuset.cpus", cpus)
		suite.AssertCgroupCPUs(nodeCtx, root, "cpuset.cpus.effective", cpus)
	}
}

// assertKubelet checks the kubelet runs the static CPU manager policy, and that a kubelet restart
// leaves the kubepods mask at its bound.
func (suite *CPUPartitionSuite) assertKubelet(nodeCtx context.Context, node string, kubepods, reserved cpuset.CPUSet) {
	if !suite.Capabilities().SupportsKubernetes {
		return
	}

	suite.Require().EventuallyWithT(func(collect *assert.CollectT) {
		reader, err := suite.Client.Read(nodeCtx, "/etc/kubernetes/kubelet.yaml")
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

	restartSince := suite.LatestServiceEventTimestamp(suite.ctx, node, "kubelet")

	_, err := suite.Client.ServiceRestart(nodeCtx, "kubelet")
	suite.Require().NoError(err)

	suite.AssertServiceEventsInOrder(suite.ctx, node, "kubelet", restartSince, []string{"Stopping", "Finished", "Starting", "Waiting", "Preparing", "Running"})

	rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.State(), "kubelet",
		func(svc *v1alpha1.Service, asrt *assert.Assertions) {
			asrt.True(svc.TypedSpec().Healthy)
		})

	suite.AssertCgroupCPUs(nodeCtx, constants.CgroupKubepods, "cpuset.cpus", kubepods)
}

func init() {
	allSuites = append(allSuites, new(CPUPartitionSuite))
}
