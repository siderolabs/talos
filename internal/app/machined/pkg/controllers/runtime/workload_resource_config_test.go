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
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

const kubernetesOnlyWorkloadResourceYAML = `apiVersion: v1alpha1
kind: WorkloadResourceConfig
kubepods:
    memory:
        limit: 50%
`

// workerReserved is the kubelet's default systemReserved.memory on workers.
const workerReserved = 384 << 20

// testCapacity leaves 10GiB to split after the worker reservation.
const testCapacity = 10<<30 + workerReserved

func newWorkloadResourceRoot(limit string) *runtimecfg.WorkloadResourceRoot {
	return &runtimecfg.WorkloadResourceRoot{
		MemoryConfig: &runtimecfg.WorkloadMemoryResource{MemoryLimit: meta.MustPercentageSize(limit)},
	}
}

func newWorkloadResourceConfig() *runtimecfg.WorkloadResourceConfigV1Alpha1 {
	doc := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	doc.KubepodsConfig = newWorkloadResourceRoot("50%")
	doc.TalosContainersConfig = newWorkloadResourceRoot("10%")
	doc.VirtualMachinesConfig = newWorkloadResourceRoot("40%")

	return doc
}

func newReserved(memory string) *runtimecfg.WorkloadReserved {
	return &runtimecfg.WorkloadReserved{MemoryConfig: meta.MustByteSize(memory)}
}

func (suite *WorkloadResourceConfigSuite) createTopology(nodes ...uint64) *hardware.NUMATopology {
	if len(nodes) == 0 {
		nodes = []uint64{testCapacity}
	}

	topology := hardware.NewNUMATopology()

	for id, bytes := range nodes {
		topology.TypedSpec().Nodes = append(topology.TypedSpec().Nodes, hardware.NUMANodeSpec{ID: uint32(id), MemoryTotalBytes: bytes})
	}

	suite.Create(topology)

	return topology
}

func newMachineDocument(machineType machine.Type) *v1alpha1.Config {
	return &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{MachineType: machineType.String()},
	}
}

// newWorkloadMemorySpec is newWorkloadResourceConfig resolved on the default worker host.
func newWorkloadMemorySpec() runtime.WorkloadMemorySpecSpec {
	return runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 1 << 30,
		VirtualMachinesLimit: 4 << 30,
		MemoryCapacity:       testCapacity,
		MemoryReserved:       workerReserved,
	}
}

// defaultWorkloadMemorySpec caps the Talos-owned roots of the default worker host at all memory beyond the default reservation.
func defaultWorkloadMemorySpec() runtime.WorkloadMemorySpecSpec {
	return runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 10 << 30,
		VirtualMachinesLimit: 10 << 30,
		MemoryCapacity:       testCapacity,
		MemoryReserved:       workerReserved,
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

func (suite *WorkloadResourceConfigSuite) getSpec() *runtime.WorkloadMemorySpec {
	spec, err := safe.StateGetByID[*runtime.WorkloadMemorySpec](suite.Ctx(), suite.State(), runtime.WorkloadMemorySpecID)
	suite.Require().NoError(err)

	return spec
}

func (suite *WorkloadResourceConfigSuite) TestPendingThenDefaultsThenPolicyThenRemoved() {
	suite.register(machineruntime.ModeMetal)

	worker := newMachineDocument(machine.TypeWorker)
	suite.createConfig(worker)

	// The limits are resolved against the host memory.
	ctest.AssertNoResource[*runtime.WorkloadMemorySpec](suite, runtime.WorkloadMemorySpecID)

	suite.createTopology()
	suite.assertSpec(defaultWorkloadMemorySpec())

	suite.replaceConfig(worker, newWorkloadResourceConfig())
	suite.assertSpec(newWorkloadMemorySpec())

	containersOnly := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	containersOnly.TalosContainersConfig = newWorkloadResourceRoot("20%")

	suite.replaceConfig(worker, containersOnly)
	// The omitted virtual machine root keeps its default.
	suite.assertSpec(runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 2 << 30, VirtualMachinesLimit: 10 << 30, MemoryCapacity: testCapacity, MemoryReserved: workerReserved,
	})

	suite.replaceConfig(worker)
	suite.assertSpec(defaultWorkloadMemorySpec())

	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)
	suite.Destroy(old)

	ctest.AssertNoResource[*runtime.WorkloadMemorySpec](suite, runtime.WorkloadMemorySpecID)
}

func (suite *WorkloadResourceConfigSuite) TestControlPlaneDefault() {
	suite.register(machineruntime.ModeMetal)
	suite.createTopology(10<<30 + 512<<20)
	suite.createConfig(newMachineDocument(machine.TypeControlPlane))

	suite.assertSpec(runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 10 << 30, VirtualMachinesLimit: 10 << 30, MemoryCapacity: 10<<30 + 512<<20, MemoryReserved: 512 << 20,
	})
}

func (suite *WorkloadResourceConfigSuite) TestKubernetesOnlyYAML() {
	suite.register(machineruntime.ModeMetal)
	suite.createTopology()

	provider, err := configloader.NewFromBytes([]byte(kubernetesOnlyWorkloadResourceYAML))
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(provider))

	expected := defaultWorkloadMemorySpec()

	suite.assertSpec(expected)
}

func (suite *WorkloadResourceConfigSuite) TestLimitsFollowTopology() {
	suite.register(machineruntime.ModeMetal)

	doc := runtimecfg.NewWorkloadResourceConfigV1Alpha1()
	doc.ReservedConfig = newReserved("2GiB")
	doc.TalosContainersConfig = newWorkloadResourceRoot("10%")
	doc.VirtualMachinesConfig = newWorkloadResourceRoot("40%")

	suite.createConfig(doc)

	// Nothing can be resolved without the host memory, so the projection stays pending.
	ctest.AssertNoResource[*runtime.WorkloadMemorySpec](suite, runtime.WorkloadMemorySpecID)

	topology := suite.createTopology(6<<30, 6<<30)

	suite.assertSpec(runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 1 << 30, VirtualMachinesLimit: 4 << 30, MemoryCapacity: 12 << 30, MemoryReserved: 2 << 30,
	})

	// Memory hotplug republishes the topology, and the limits follow it once it settles.
	ctest.UpdateWithConflicts(suite, topology, func(res *hardware.NUMATopology) error {
		res.TypedSpec().Nodes[1].MemoryTotalBytes = 16 << 30

		return nil
	})

	suite.assertSpec(runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 2 << 30, VirtualMachinesLimit: 8 << 30, MemoryCapacity: 22 << 30, MemoryReserved: 2 << 30,
	})

	// The inventory is briefly gone while hotplug races a snapshot: the last limits stay published.
	suite.Destroy(topology)

	suite.assertSpec(runtime.WorkloadMemorySpecSpec{
		TalosContainersLimit: 2 << 30, VirtualMachinesLimit: 8 << 30, MemoryCapacity: 22 << 30, MemoryReserved: 2 << 30,
	})
}

func (suite *WorkloadResourceConfigSuite) TestDefaultsNeedMemoryBeyondReservation() {
	suite.register(machineruntime.ModeMetal)
	suite.createTopology(256 << 20)
	suite.createConfig(newMachineDocument(machine.TypeWorker))

	// A one-byte cap could not be applied, so the omitted roots stay unlimited.
	suite.assertSpec(runtime.WorkloadMemorySpecSpec{MemoryCapacity: 256 << 20, MemoryReserved: workerReserved})
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
		suite.createTopology()

		suite.createConfig(newWatchdogDocument("/dev/watchdog0"), newWorkloadResourceConfig())
		synctest.Wait()

		spec := suite.getSpec()
		require.Equal(t, newWorkloadMemorySpec(), *spec.TypedSpec())

		version := spec.Metadata().Version()

		// The explicit default reservation and an unrelated document change resolve to the same limits.
		rewritten := newWorkloadResourceConfig()
		rewritten.ReservedConfig = newReserved("384MiB")

		suite.replaceConfig(newWatchdogDocument("/dev/watchdog1"), rewritten)
		synctest.Wait()

		assert.Equal(t, version, suite.getSpec().Metadata().Version())

		changed := newWorkloadResourceConfig()
		changed.VirtualMachinesConfig = newWorkloadResourceRoot("20%")

		suite.replaceConfig(newWatchdogDocument("/dev/watchdog1"), changed)
		synctest.Wait()

		spec = suite.getSpec()
		assert.Equal(t, uint64(2<<30), spec.TypedSpec().VirtualMachinesLimit)
		assert.Equal(t, version.Next(), spec.Metadata().Version())
	})
}

func TestWorkloadResourceConfigCapacityBurstSettles(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		suite := &WorkloadResourceConfigSuite{Timeout: time.Minute}
		suite.SetT(t)
		suite.SetupTest()

		defer suite.TearDownTest()

		suite.register(machineruntime.ModeMetal)
		topology := suite.createTopology()

		suite.createConfig(newWorkloadResourceConfig())
		synctest.Wait()

		version := suite.getSpec().Metadata().Version()

		// Three memory blocks go offline one second apart, one topology update each.
		for range 3 {
			topology = ctest.UpdateWithConflicts(suite, topology, func(res *hardware.NUMATopology) error {
				res.TypedSpec().Nodes[0].MemoryTotalBytes -= 128 << 20

				return nil
			})

			synctest.Wait()
			assert.Equal(t, version, suite.getSpec().Metadata().Version(), "a capacity change must wait to settle")

			time.Sleep(time.Second)
		}

		time.Sleep(runtimectrls.CapacitySettleDelay)
		synctest.Wait()

		// The burst lands as a single update, resolved against the final capacity.
		spec := suite.getSpec()
		assert.Equal(t, version.Next(), spec.Metadata().Version())
		assert.Equal(t, uint64(testCapacity-3*128<<20), spec.TypedSpec().MemoryCapacity)

		// A configuration change is not held back.
		changed := newWorkloadResourceConfig()
		changed.VirtualMachinesConfig = newWorkloadResourceRoot("20%")

		suite.replaceConfig(changed)
		synctest.Wait()

		assert.Equal(t, version.Next().Next(), suite.getSpec().Metadata().Version())
	})
}
