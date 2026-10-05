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
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const kubernetesOnlyWorkloadResourceYAML = `apiVersion: v1alpha1
kind: WorkloadResourceConfig
kubepods:
    memory:
        limit: 16GiB
`

func newWorkloadResourceRoot(limit string) *runtimecfg.WorkloadResourceRoot {
	return &runtimecfg.WorkloadResourceRoot{
		MemoryConfig: &runtimecfg.WorkloadMemoryResource{MemoryLimit: meta.MustByteSize(limit)},
	}
}

func newWorkloadResourceConfig() *runtimecfg.WorkloadResourceConfigV1Alpha1 {
	doc := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	doc.KubepodsConfig = newWorkloadResourceRoot("16GiB")
	doc.TalosContainersConfig = newWorkloadResourceRoot("4GiB")
	doc.VirtualMachinesConfig = newWorkloadResourceRoot("32GiB")

	return doc
}

func newWorkloadMemorySpec() runtime.WorkloadMemorySpecSpec {
	return runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 4 << 30,
		VirtualMachinesLimit: 32 << 30,
	}
}

type WorkloadResourceConfigSuite struct {
	ctest.DefaultSuite
}

func TestWorkloadResourceConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &WorkloadResourceConfigSuite{
		Timeout: 30 * time.Second,
	})
}

func (suite *WorkloadResourceConfigSuite) register(mode machineruntime.Mode) {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.WorkloadResourceConfigController{
		V1Alpha1Mode: mode,
	}))
}

func (suite *WorkloadResourceConfigSuite) createConfig(docs ...configcfg.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(cfg))
}

func (suite *WorkloadResourceConfigSuite) replaceConfig(docs ...configcfg.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)

	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)

	replacement := config.NewMachineConfig(cfg)
	replacement.Metadata().SetVersion(old.Metadata().Version())
	suite.Update(replacement)
}

func (suite *WorkloadResourceConfigSuite) assertSpec(expected runtime.WorkloadMemorySpecSpec) {
	ctest.AssertResource(suite, runtime.WorkloadMemorySpecID, func(res *runtime.WorkloadMemorySpec, asrt *assert.Assertions) {
		asrt.Equal(expected, *res.TypedSpec())
		asrt.Equal("runtime.WorkloadResourceConfigController", res.Metadata().Owner())
	})
}

func (suite *WorkloadResourceConfigSuite) TestPendingThenEmptyThenPolicyThenRemoved() {
	suite.register(machineruntime.ModeMetal)

	ctest.AssertNoResource[*runtime.WorkloadMemorySpec](suite, runtime.WorkloadMemorySpecID)

	watchdog := newWatchdogDocument("/dev/watchdog0")
	suite.createConfig(watchdog)
	suite.assertSpec(runtime.WorkloadMemorySpecSpec{})

	suite.replaceConfig(watchdog, newWorkloadResourceConfig())
	suite.assertSpec(newWorkloadMemorySpec())

	containersOnly := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	containersOnly.TalosContainersConfig = newWorkloadResourceRoot("2GiB")

	suite.replaceConfig(watchdog, containersOnly)
	suite.assertSpec(runtime.WorkloadMemorySpecSpec{TalosContainersLimit: 2 << 30})

	suite.replaceConfig(watchdog)
	suite.assertSpec(runtime.WorkloadMemorySpecSpec{})

	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)
	suite.Destroy(old)

	ctest.AssertNoResource[*runtime.WorkloadMemorySpec](suite, runtime.WorkloadMemorySpecID)
}

func (suite *WorkloadResourceConfigSuite) TestKubernetesOnlyYAML() {
	suite.register(machineruntime.ModeMetal)

	provider, err := configloader.NewFromBytes([]byte(kubernetesOnlyWorkloadResourceYAML))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(provider))

	suite.assertSpec(runtime.WorkloadMemorySpecSpec{})
}

func (suite *WorkloadResourceConfigSuite) TestContainerModeIgnoresPolicy() {
	suite.register(machineruntime.ModeContainer)

	suite.createConfig(newWorkloadResourceConfig())
	suite.assertSpec(runtime.WorkloadMemorySpecSpec{})
}

func TestWorkloadResourceConfigUnchangedProjectionKeepsVersion(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		suite := &WorkloadResourceConfigSuite{Timeout: 30 * time.Second}
		suite.SetT(t)
		suite.SetupTest()

		defer suite.TearDownTest()

		suite.register(machineruntime.ModeMetal)

		suite.createConfig(newWatchdogDocument("/dev/watchdog0"), newWorkloadResourceConfig())
		synctest.Wait()

		spec, err := safe.StateGetByID[*runtime.WorkloadMemorySpec](suite.Ctx(), suite.State(), runtime.WorkloadMemorySpecID)
		require.NoError(t, err)
		require.Equal(t, newWorkloadMemorySpec(), *spec.TypedSpec())

		version := spec.Metadata().Version()

		rewritten := newWorkloadResourceConfig()
		rewritten.KubepodsConfig = newWorkloadResourceRoot("8GiB")
		rewritten.VirtualMachinesConfig = newWorkloadResourceRoot("32768MiB")

		suite.replaceConfig(newWatchdogDocument("/dev/watchdog1"), rewritten)
		synctest.Wait()

		spec, err = safe.StateGetByID[*runtime.WorkloadMemorySpec](suite.Ctx(), suite.State(), runtime.WorkloadMemorySpecID)
		require.NoError(t, err)
		assert.Equal(t, version, spec.Metadata().Version())

		changed := newWorkloadResourceConfig()
		changed.VirtualMachinesConfig = newWorkloadResourceRoot("16GiB")

		suite.replaceConfig(newWatchdogDocument("/dev/watchdog1"), changed)
		synctest.Wait()

		spec, err = safe.StateGetByID[*runtime.WorkloadMemorySpec](suite.Ctx(), suite.State(), runtime.WorkloadMemorySpecID)
		require.NoError(t, err)
		assert.Equal(t, uint64(16<<30), spec.TypedSpec().VirtualMachinesLimit)
		assert.Equal(t, version.Next(), spec.Metadata().Version())
	})
}
