// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const kubernetesOnlyCPUPartitionYAML = `apiVersion: v1alpha1
kind: CPUPartitionConfig
init:
    cpus: 1,0
system:
    cpus: 0-1
podruntime:
    cpus: 0,1
kubepods:
    cpus: 2-7,3
`

func newCPUPartitionConfig() *runtimecfg.CPUPartitionConfigV1Alpha1 {
	doc := runtimecfg.NewCPUPartitionConfigV1Alpha1()
	doc.InitConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.SystemConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.PodRuntimeConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.KubepodsConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "2-3"}
	doc.TalosContainersConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.VirtualMachinesConfig = &runtimecfg.CPUPartitionVirtualMachines{
		RootCPUs: "4-7",
		SlicesConfig: []runtimecfg.CPUPartitionSlice{
			{SliceName: "database", SliceCPUs: "5,4", SliceExclusive: new(true)},
			{SliceName: "batch", SliceCPUs: "6-7"},
		},
	}

	return doc
}

func newCPUPartitionSpec() runtime.CPUPartitionSpecSpec {
	return runtime.CPUPartitionSpecSpec{
		Enabled: true,
		Roots: map[string]string{
			"init":            "0-1",
			"system":          "0-1",
			"podruntime":      "0-1",
			"kubepods":        "2-3",
			"taloscontainers": "0-1",
			"virtualmachines": "4-7",
		},
		Slices: []runtime.CPUPartitionSliceSpec{
			{Name: "database", CPUs: "4-5", Exclusive: true},
			{Name: "batch", CPUs: "6-7"},
		},
	}
}

func newWatchdogDocument(device string) *runtimecfg.WatchdogTimerV1Alpha1 {
	watchdog := runtimecfg.NewWatchdogTimerV1Alpha1()
	watchdog.WatchdogDevice = device

	return watchdog
}

type CPUPartitionConfigSuite struct {
	ctest.DefaultSuite
}

func TestCPUPartitionConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CPUPartitionConfigSuite{
		Timeout: 30 * time.Second,
	})
}

func (suite *CPUPartitionConfigSuite) register(mode machineruntime.Mode) {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.CPUPartitionConfigController{
		V1Alpha1Mode: mode,
	}))
}

func (suite *CPUPartitionConfigSuite) createConfig(docs ...configcfg.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(cfg))
}

func (suite *CPUPartitionConfigSuite) replaceConfig(docs ...configcfg.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)

	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)

	replacement := config.NewMachineConfig(cfg)
	replacement.Metadata().SetVersion(old.Metadata().Version())
	suite.Update(replacement)
}

func (suite *CPUPartitionConfigSuite) assertSpec(expected runtime.CPUPartitionSpecSpec) {
	ctest.AssertResource(suite, runtime.CPUPartitionSpecID, func(res *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
		asrt.Equal(expected, *res.TypedSpec())
		asrt.Equal("runtime.CPUPartitionConfigController", res.Metadata().Owner())
	})
}

func (suite *CPUPartitionConfigSuite) TestPendingThenDisabledThenPolicyThenRemoved() {
	suite.register(machineruntime.ModeMetal)

	ctest.AssertNoResource[*runtime.CPUPartitionSpec](suite, runtime.CPUPartitionSpecID)

	watchdog := newWatchdogDocument("/dev/watchdog0")
	suite.createConfig(watchdog)
	suite.assertSpec(runtime.CPUPartitionSpecSpec{})

	suite.replaceConfig(watchdog, newCPUPartitionConfig())
	suite.assertSpec(newCPUPartitionSpec())

	suite.replaceConfig(watchdog)
	suite.assertSpec(runtime.CPUPartitionSpecSpec{})

	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)
	suite.Destroy(old)

	ctest.AssertNoResource[*runtime.CPUPartitionSpec](suite, runtime.CPUPartitionSpecID)
}

func (suite *CPUPartitionConfigSuite) TestKubernetesOnlyYAML() {
	suite.register(machineruntime.ModeMetal)

	provider, err := configloader.NewFromBytes([]byte(kubernetesOnlyCPUPartitionYAML))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(provider))

	suite.assertSpec(runtime.CPUPartitionSpecSpec{
		Enabled: true,
		Roots: map[string]string{
			"init":       "0-1",
			"system":     "0-1",
			"podruntime": "0-1",
			"kubepods":   "2-7",
		},
	})
}

func (suite *CPUPartitionConfigSuite) TestContainerModeIgnoresPolicy() {
	suite.register(machineruntime.ModeContainer)

	suite.createConfig(newCPUPartitionConfig())
	suite.assertSpec(runtime.CPUPartitionSpecSpec{})
}

func TestCPUPartitionConfigUnchangedProjectionKeepsVersion(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		suite := &CPUPartitionConfigSuite{Timeout: 30 * time.Second}
		suite.SetT(t)
		suite.SetupTest()

		defer suite.TearDownTest()

		suite.register(machineruntime.ModeMetal)

		suite.createConfig(newWatchdogDocument("/dev/watchdog0"), newCPUPartitionConfig())
		synctest.Wait()

		spec, err := safe.StateGetByID[*runtime.CPUPartitionSpec](suite.Ctx(), suite.State(), runtime.CPUPartitionSpecID)
		require.NoError(t, err)
		require.Equal(t, newCPUPartitionSpec(), *spec.TypedSpec())

		version := spec.Metadata().Version()

		rewritten := newCPUPartitionConfig()
		rewritten.InitConfig.RootCPUs = "0,1"
		rewritten.VirtualMachinesConfig.SlicesConfig[0].SliceCPUs = "4-5"

		suite.replaceConfig(newWatchdogDocument("/dev/watchdog1"), rewritten)
		synctest.Wait()

		spec, err = safe.StateGetByID[*runtime.CPUPartitionSpec](suite.Ctx(), suite.State(), runtime.CPUPartitionSpecID)
		require.NoError(t, err)
		assert.Equal(t, version, spec.Metadata().Version())

		changed := newCPUPartitionConfig()
		changed.KubepodsConfig.RootCPUs = "2"

		suite.replaceConfig(newWatchdogDocument("/dev/watchdog1"), changed)
		synctest.Wait()

		spec, err = safe.StateGetByID[*runtime.CPUPartitionSpec](suite.Ctx(), suite.State(), runtime.CPUPartitionSpecID)
		require.NoError(t, err)
		assert.Equal(t, "2", spec.TypedSpec().Roots["kubepods"])
		assert.Equal(t, version.Next(), spec.Metadata().Version())
	})
}
