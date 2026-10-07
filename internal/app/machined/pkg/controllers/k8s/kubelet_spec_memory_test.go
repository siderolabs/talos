// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	"errors"
	"net/netip"
	"strconv"
	"sync/atomic"
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
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// fakeMemoryCapacity records how often the controller asks for the host memory capacity.
type fakeMemoryCapacity struct {
	capacity atomic.Uint64
	err      atomic.Pointer[error]
	reads    atomic.Int64
}

func (f *fakeMemoryCapacity) read() (uint64, error) {
	f.reads.Add(1)

	if err := f.err.Load(); err != nil {
		return 0, *err
	}

	return f.capacity.Load(), nil
}

type KubeletSpecMemorySuite struct {
	ctest.DefaultSuite

	mode     v1alpha1runtime.Mode
	memory   *fakeMemoryCapacity
	logs     *observer.ObservedLogs
	logsCore zapcore.Core
}

func TestKubeletSpecMemorySuite(t *testing.T) {
	t.Parallel()

	for _, mode := range []v1alpha1runtime.Mode{v1alpha1runtime.ModeMetal, v1alpha1runtime.ModeContainer} {
		t.Run(mode.String(), func(t *testing.T) {
			t.Parallel()

			suite.Run(t, newKubeletSpecMemorySuite(mode))
		})
	}
}

func newKubeletSpecMemorySuite(mode v1alpha1runtime.Mode) *KubeletSpecMemorySuite {
	logsCore, logs := observer.New(zapcore.ErrorLevel)

	s := &KubeletSpecMemorySuite{
		mode:     mode,
		memory:   &fakeMemoryCapacity{},
		logs:     logs,
		logsCore: logsCore,
	}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		Logger:  zap.New(logsCore),
		AfterSetup: func(ds *ctest.DefaultSuite) {
			s.memory.capacity.Store(testMemoryCapacity)
			s.memory.err.Store(nil)
			s.memory.reads.Store(0)
			logs.TakeAll()

			ds.Require().NoError(ds.Runtime().RegisterController(&k8sctrl.KubeletSpecController{
				V1Alpha1Mode:   mode,
				MemoryCapacity: s.memory.read,
			}))
		},
	}

	return s
}

func (suite *KubeletSpecMemorySuite) createInputs(limit uint64, extraConfig map[string]any) *k8s.KubeletConfig {
	cfg := k8s.NewKubeletConfig(k8s.NamespaceName, k8s.KubeletID)
	cfg.TypedSpec().Image = "kubelet:v1.38.0-alpha.1"
	cfg.TypedSpec().ClusterDNS = []string{"10.96.0.10"}
	cfg.TypedSpec().ClusterDomain = "cluster.local"
	cfg.TypedSpec().ExtraConfig = extraConfig
	cfg.TypedSpec().KubepodsMemoryLimit = limit

	suite.Create(cfg)

	nodeIP := k8s.NewNodeIP(k8s.NamespaceName, k8s.KubeletID)
	nodeIP.TypedSpec().Addresses = []netip.Addr{netip.MustParseAddr("172.20.0.2")}
	suite.Create(nodeIP)

	nodename := k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID)
	nodename.TypedSpec().Nodename = "example.com"
	suite.Create(nodename)

	machineType := config.NewMachineType()
	machineType.SetMachineType(machine.TypeWorker)
	suite.Create(machineType)

	return cfg
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

func (suite *KubeletSpecMemorySuite) renderFailures() []observer.LoggedEntry {
	return suite.logs.FilterMessage("controller failed").FilterFieldKey("error").All()
}

func defaultSystemReserved(memory string) map[string]any {
	return map[string]any{
		"cpu":               constants.KubeletSystemReservedCPU,
		"pid":               constants.KubeletSystemReservedPid,
		"ephemeral-storage": constants.KubeletSystemReservedEphemeralStorage,
		"memory":            memory,
	}
}

func (suite *KubeletSpecMemorySuite) TestNoLimitNeverReadsCapacity() {
	suite.createInputs(0, map[string]any{"systemReserved": map[string]any{"memory": "1Gi"}, "cgroupsPerQOS": false})

	suite.assertSystemReserved(map[string]any{"memory": "1Gi"}, nil)

	suite.Assert().Zero(suite.memory.reads.Load())
	suite.Assert().Empty(suite.renderFailures())
}

func (suite *KubeletSpecMemorySuite) TestLimitLifecycle() {
	if suite.mode == v1alpha1runtime.ModeContainer {
		suite.T().Skip("the limit is inactive in container mode")
	}

	cfg := suite.createInputs(testKubepodsLimit, nil)

	managed := strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10)

	var managedVersion resource.Version

	suite.assertSystemReserved(defaultSystemReserved(managed), func(spec *k8s.KubeletSpecSpec, asrt *assert.Assertions) {
		asrt.Equal("cluster.local", spec.Config["clusterDomain"])
	})

	rendered, err := ctest.Get[*k8s.KubeletSpec](suite, k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID).Metadata())
	suite.Require().NoError(err)

	managedVersion = rendered.Metadata().Version()

	suite.Assert().Positive(suite.memory.reads.Load())

	// a rejected update keeps the last rendered spec and surfaces the reason
	cfg = suite.updateConfig(cfg, func(spec *k8s.KubeletConfigSpec) {
		spec.KubepodsMemoryLimit = testMemoryCapacity + testPageSize
		spec.ClusterDomain = "rejected.local"
	})

	suite.AssertWithin(5*time.Second, 10*time.Millisecond, func() error {
		for _, entry := range suite.renderFailures() {
			if err, ok := entry.ContextMap()["error"].(string); ok &&
				err == `error creating kubelet configuration: kubepods memory limit 68719480832 bytes exceeds the host memory capacity 68719476736 bytes minus "kubeReserved.memory" 0 bytes` {
				return nil
			}
		}

		return retry.ExpectedErrorf("rejection not logged yet")
	})

	rendered, err = ctest.Get[*k8s.KubeletSpec](suite, k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID).Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(managedVersion, rendered.Metadata().Version())
	suite.Assert().Equal(defaultSystemReserved(managed), rendered.TypedSpec().Config["systemReserved"])
	suite.Assert().Equal("cluster.local", rendered.TypedSpec().Config["clusterDomain"])

	// a static conflict is rejected before the capacity is read or any defaults are applied
	readsBeforeConflict := suite.memory.reads.Load()

	cfg = suite.updateConfig(cfg, func(spec *k8s.KubeletConfigSpec) {
		spec.KubepodsMemoryLimit = testKubepodsLimit
		spec.ExtraConfig = map[string]any{"systemReserved": map[string]any{"memory": "1Gi"}}
	})

	suite.AssertWithin(5*time.Second, 10*time.Millisecond, func() error {
		for _, entry := range suite.renderFailures() {
			if err, ok := entry.ContextMap()["error"].(string); ok &&
				err == `error creating kubelet configuration: kubelet configuration field "systemReserved.memory" conflicts with the kubepods memory limit: Talos derives it from the limit` {
				return nil
			}
		}

		return retry.ExpectedErrorf("conflict not logged yet")
	})

	rendered, err = ctest.Get[*k8s.KubeletSpec](suite, k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID).Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(managedVersion, rendered.Metadata().Version())
	suite.Assert().Equal(readsBeforeConflict, suite.memory.reads.Load())

	// a changed limit and other settings from the same revision land in one update
	cfg = suite.updateConfig(cfg, func(spec *k8s.KubeletConfigSpec) {
		spec.KubepodsMemoryLimit = testKubepodsLimit / 2
		spec.ExtraConfig = map[string]any{"systemReserved": map[string]any{"cpu": "1"}, "kubeReserved": map[string]any{"memory": "1Gi"}}
		spec.ClusterDomain = "updated.local"
	})

	suite.assertSystemReserved(map[string]any{"cpu": "1", "memory": strconv.FormatUint(testMemoryCapacity-testKubepodsLimit/2-(1<<30), 10)}, func(spec *k8s.KubeletSpecSpec, asrt *assert.Assertions) {
		asrt.Equal("updated.local", spec.Config["clusterDomain"])
	})

	rendered, err = ctest.Get[*k8s.KubeletSpec](suite, k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID).Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(managedVersion.Next(), rendered.Metadata().Version())

	// removing the limit restores the user configuration with Talos defaults, never a stale reservation
	suite.updateConfig(cfg, func(spec *k8s.KubeletConfigSpec) {
		spec.KubepodsMemoryLimit = 0
		spec.ExtraConfig = nil
		spec.ClusterDomain = "cluster.local"
	})

	suite.assertSystemReserved(defaultSystemReserved(constants.KubeletSystemReservedMemoryWorker), func(spec *k8s.KubeletSpecSpec, asrt *assert.Assertions) {
		asrt.Equal("cluster.local", spec.Config["clusterDomain"])
	})
}

func (suite *KubeletSpecMemorySuite) TestCapacityFailureKeepsRenderedSpec() {
	if suite.mode == v1alpha1runtime.ModeContainer {
		suite.T().Skip("the limit is inactive in container mode")
	}

	cfg := suite.createInputs(0, nil)

	suite.assertSystemReserved(defaultSystemReserved(constants.KubeletSystemReservedMemoryWorker), nil)

	readErr := errors.New("meminfo unavailable")
	suite.memory.err.Store(&readErr)

	suite.updateConfig(cfg, func(spec *k8s.KubeletConfigSpec) {
		spec.KubepodsMemoryLimit = testKubepodsLimit
	})

	suite.AssertWithin(5*time.Second, 10*time.Millisecond, func() error {
		for _, entry := range suite.renderFailures() {
			if err, ok := entry.ContextMap()["error"].(string); ok &&
				err == "error creating kubelet configuration: error reading memory capacity for the kubepods memory limit: meminfo unavailable" {
				return nil
			}
		}

		return retry.ExpectedErrorf("capacity failure not logged yet")
	})

	rendered, err := ctest.Get[*k8s.KubeletSpec](suite, k8s.NewKubeletSpec(k8s.NamespaceName, k8s.KubeletID).Metadata())
	suite.Require().NoError(err)
	suite.Assert().Equal(defaultSystemReserved(constants.KubeletSystemReservedMemoryWorker), rendered.TypedSpec().Config["systemReserved"])

	// once the capacity is readable again the controller recovers on its own
	suite.memory.err.Store(nil)

	suite.assertSystemReserved(defaultSystemReserved(strconv.FormatUint(testMemoryCapacity-testKubepodsLimit, 10)), nil)
}

func (suite *KubeletSpecMemorySuite) TestContainerModeIgnoresLimit() {
	if suite.mode != v1alpha1runtime.ModeContainer {
		suite.T().Skip("only applies to container mode")
	}

	suite.createInputs(testMemoryCapacity*4, map[string]any{"systemReserved": map[string]any{"memory": "1Gi"}, "cgroupsPerQOS": false})

	suite.assertSystemReserved(map[string]any{"memory": "1Gi"}, func(spec *k8s.KubeletSpecSpec, asrt *assert.Assertions) {
		asrt.NotContains(spec.Config, "protectKernelDefaults")
	})

	suite.Assert().Zero(suite.memory.reads.Load())
	suite.Assert().Empty(suite.renderFailures())
}
