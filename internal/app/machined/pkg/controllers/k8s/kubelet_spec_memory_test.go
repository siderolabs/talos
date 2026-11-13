// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"net/netip"
	"strconv"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	k8sctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s"
	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type KubeletSpecMemorySuite struct {
	ctest.DefaultSuite

	logs *observer.ObservedLogs
}

func TestKubeletSpecMemorySuite(t *testing.T) {
	t.Parallel()

	logsCore, logs := observer.New(zapcore.ErrorLevel)

	s := &KubeletSpecMemorySuite{logs: logs}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		Logger:  zap.New(logsCore),
		AfterSetup: func(ds *ctest.DefaultSuite) {
			logs.TakeAll()

			ds.Require().NoError(ds.Runtime().RegisterController(&k8sctrl.KubeletSpecController{
				V1Alpha1Mode: v1alpha1runtime.ModeMetal,
			}))
		},
	}

	suite.Run(t, s)
}

func (suite *KubeletSpecMemorySuite) createInputs(extraConfig map[string]any, reserved, kubepodsPercent uint64) *k8s.KubeletConfig {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.38.0-alpha.1"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraConfig = extraConfig
	cfg.TypedSpec().MemoryReserved = reserved
	cfg.TypedSpec().KubepodsMemoryPercent = kubepodsPercent

	suite.Create(cfg)

	nodeIP := k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID)
	nodeIP.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("172.20.0.2")}
	suite.Create(nodeIP)

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example.com"
	suite.Create(nodename)

	return cfg
}

func (suite *KubeletSpecMemorySuite) createWorkloadMemory(spec runtime.WorkloadMemorySpecSpec) *runtime.WorkloadMemorySpec {
	workloadMemory := runtime.NewWorkloadMemorySpec()
	*workloadMemory.TypedSpec() = spec

	suite.Create(workloadMemory)

	return workloadMemory
}

func (suite *KubeletSpecMemorySuite) updateWorkloadMemory(workloadMemory *runtime.WorkloadMemorySpec, spec runtime.WorkloadMemorySpecSpec) *runtime.WorkloadMemorySpec {
	return ctest.UpdateWithConflicts(suite, workloadMemory, func(r *runtime.WorkloadMemorySpec) error {
		*r.TypedSpec() = spec

		return nil
	})
}

func (suite *KubeletSpecMemorySuite) updateConfig(cfg *k8s.KubeletConfig, update func(*k8s.KubeletConfigSpec)) *k8s.KubeletConfig {
	return ctest.UpdateWithConflicts(suite, cfg, func(r *k8s.KubeletConfig) error {
		update(r.TypedSpec())

		return nil
	})
}

func (suite *KubeletSpecMemorySuite) assertSystemReserved(expected map[string]any, extra func(spec *k8s.KubeletSpecSpec, asrt *assert.Assertions)) {
	ctest.AssertResource(suite, k8s.KubeletID, func(r *k8s.KubeletSpec, asrt *assert.Assertions) {
		spec := r.TypedSpec()

		asrt.Equal(expected, spec.Config["systemReserved"])

		if extra != nil {
			extra(spec, asrt)
		}
	})
}

func (suite *KubeletSpecMemorySuite) assertRenderFailure(expected string) {
	suite.AssertWithin(5*time.Second, 10*time.Millisecond, func() error {
		for _, entry := range suite.logs.FilterMessage("controller failed").FilterFieldKey("error").All() {
			if err, ok := entry.ContextMap()["error"].(string); ok && err == expected {
				return nil
			}
		}

		return retry.ExpectedErrorf("%q not logged yet", expected)
	})
}

func (suite *KubeletSpecMemorySuite) renderedVersion() resource.Version {
	rendered, err := ctest.Get[*k8s.KubeletSpec](suite, k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID).Metadata())
	suite.Require().NoError(err)

	return rendered.Metadata().Version()
}

func defaultSystemReserved(memory string) map[string]any {
	return map[string]any{
		"cpu":               constants.KubeletSystemReservedCPU,
		"pid":               constants.KubeletSystemReservedPid,
		"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
		"memory":            memory,
	}
}

// limitReserved leaves testKubepodsLimit beyond the reservation of testMemoryCapacity, so 100% resolves to it.
const limitReserved = testMemoryCapacity - testKubepodsLimit

func (suite *KubeletSpecMemorySuite) TestReservedNeedsNoHostMemory() {
	suite.createInputs(nil, 2<<30, 0)

	// a reservation does not depend on the host memory, so it renders without WorkloadMemorySpec
	suite.assertSystemReserved(defaultSystemReserved(strconv.FormatUint(2<<30, 10)), nil)

	version := suite.renderedVersion()

	// memory hotplug changes nothing the kubelet consumes
	workloadMemory := suite.createWorkloadMemory(runtime.WorkloadMemorySpecSpec{MemoryCapacity: testMemoryCapacity, MemoryReserved: 2 << 30})
	suite.updateWorkloadMemory(workloadMemory, runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 1 << 30, MemoryCapacity: 2 * testMemoryCapacity, MemoryReserved: 2 << 30})

	suite.Assert().Equal(version, suite.renderedVersion())
}

func (suite *KubeletSpecMemorySuite) TestUserReservedWithoutLimit() {
	// without a kubepods limit the user settings win, conflicting ones included
	suite.createInputs(map[string]any{"systemReserved": map[string]any{"memory": "1Gi"}, "cgroupsPerQOS": false}, testWorkerReserved, 0)

	suite.assertSystemReserved(map[string]any{"memory": "1Gi"}, nil)
}

func (suite *KubeletSpecMemorySuite) TestLimitWaitsForHostMemory() {
	suite.createInputs(nil, limitReserved, 100)

	ctest.AssertNoResource[*k8s.KubeletSpec](suite, k8s.KubeletID)

	suite.createWorkloadMemory(runtime.WorkloadMemorySpecSpec{MemoryCapacity: testMemoryCapacity, MemoryReserved: limitReserved})

	suite.assertSystemReserved(defaultSystemReserved(strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10)), nil)
}

func (suite *KubeletSpecMemorySuite) TestLimitLifecycle() {
	capacity := func(capacity uint64) runtime.WorkloadMemorySpecSpec {
		return runtime.WorkloadMemorySpecSpec{MemoryCapacity: capacity, MemoryReserved: limitReserved}
	}

	cfg := suite.createInputs(nil, limitReserved, 100)
	workloadMemory := suite.createWorkloadMemory(capacity(testMemoryCapacity))

	suite.assertSystemReserved(defaultSystemReserved(strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10)), nil)

	managedVersion := suite.renderedVersion()

	// a rejected limit keeps the last rendered spec and surfaces the reason
	workloadMemory = suite.updateWorkloadMemory(workloadMemory, capacity(limitReserved+testPageSize-1))

	suite.assertRenderFailure(`error creating kubelet configuration: invalid kubepods memory limit: memory limit 4095 bytes is smaller than the page size of 4096 bytes`)
	suite.Assert().Equal(managedVersion, suite.renderedVersion())

	// a static conflict is rejected before the limit is resolved
	cfg = suite.updateConfig(cfg, func(spec *k8s.KubeletConfigSpec) {
		spec.ExtraConfig = map[string]any{"systemReserved": map[string]any{"memory": "1Gi"}}
	})

	suite.assertRenderFailure(`error creating kubelet configuration: kubelet configuration field "systemReserved.memory" conflicts with the kubepods memory limit: Talos derives it from the limit`)
	suite.Assert().Equal(managedVersion, suite.renderedVersion())

	// the settings change is held back until the limit is valid again, then lands in one update
	cfg = suite.updateConfig(cfg, func(spec *k8s.KubeletConfigSpec) {
		spec.ExtraConfig = map[string]any{"systemReserved": map[string]any{"cpu": "1"}}
		spec.ClusterDomain = "updated.local"
		spec.KubepodsMemoryPercent = 50
	})

	workloadMemory = suite.updateWorkloadMemory(workloadMemory, capacity(testMemoryCapacity))

	suite.assertSystemReserved(map[string]any{"cpu": "1", "memory": strconv.FormatUint(testMemoryCapacity-testKubepodsLimit/2, 10)}, func(spec *k8s.KubeletSpecSpec, asrt *assert.Assertions) {
		asrt.Equal("updated.local", spec.Config["clusterDomain"])
	})

	suite.Assert().Equal(managedVersion.Next(), suite.renderedVersion())

	// memory hotplug moves the capacity, and the limit and the reservation with it
	workloadMemory = suite.updateWorkloadMemory(workloadMemory, capacity(2*testMemoryCapacity))

	hotplugLimit := (2*testMemoryCapacity - limitReserved) / 2

	suite.assertSystemReserved(map[string]any{"cpu": "1", "memory": strconv.FormatUint(2*testMemoryCapacity-hotplugLimit, 10)}, nil)

	// removing the limit falls back to the reservation, never a stale derived value
	suite.updateConfig(cfg, func(spec *k8s.KubeletConfigSpec) {
		spec.ExtraConfig = nil
		spec.ClusterDomain = "cluster.local"
		spec.KubepodsMemoryPercent = 0
	})

	suite.updateWorkloadMemory(workloadMemory, capacity(testMemoryCapacity))

	suite.assertSystemReserved(defaultSystemReserved(strconv.FormatUint(limitReserved, 10)), func(spec *k8s.KubeletSpecSpec, asrt *assert.Assertions) {
		asrt.Equal("cluster.local", spec.Config["clusterDomain"])
	})
}
