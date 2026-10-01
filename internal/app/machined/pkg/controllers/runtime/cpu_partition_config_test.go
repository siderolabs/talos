// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimectrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimecfg "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type CPUPartitionConfigSuite struct {
	ctest.DefaultSuite
}

func TestCPUPartitionConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CPUPartitionConfigSuite{
		Timeout: 30 * time.Second,
	})
}

func newCPUPartitionConfig() *runtimecfg.CPUPartitionConfigV1Alpha1 {
	doc := runtimecfg.NewCPUPartitionConfigV1Alpha1()
	doc.InitConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "1,0"}
	doc.SystemConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.PodRuntimeConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.KubepodsConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "2-3"}
	doc.TalosContainersConfig = &runtimecfg.CPUPartitionRoot{RootCPUs: "0-1"}
	doc.VirtualMachinesConfig = &runtimecfg.CPUPartitionVirtualMachines{
		RootCPUs: "4-7",
		SlicesConfig: []runtimecfg.CPUPartitionSlice{
			{SliceName: "database", SliceCPUs: "5,4", SliceExclusive: new(true)},
		},
	}

	return doc
}

// acceptedSpec is the canonical projection of newCPUPartitionConfig.
func acceptedSpec() runtime.CPUPartitionSpecSpec {
	return runtime.CPUPartitionSpecSpec{
		Enabled: true,
		Roots: map[string]string{
			"init":            "0-1",
			"system":          "0-1",
			"podruntime":      "0-1",
			"kubepods":        "2-3",
			"taloscontainers": "0-1",
			"virtualMachines": "4-7",
		},
		Slices: []runtime.CPUPartitionSliceSpec{
			{Name: "database", CPUs: "4-5", Exclusive: true},
		},
	}
}

func (suite *CPUPartitionConfigSuite) replaceConfig(docs ...configcfg.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)

	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)

	res := config.NewMachineConfig(cfg)
	res.Metadata().SetVersion(old.Metadata().Version())
	suite.Update(res)
}

func (suite *CPUPartitionConfigSuite) specVersion() resource.Version {
	spec, err := safe.StateGetByID[*runtime.CPUPartitionSpec](suite.Ctx(), suite.State(), runtime.CPUPartitionSpecID)
	suite.Require().NoError(err)

	return spec.Metadata().Version()
}

func (suite *CPUPartitionConfigSuite) TestProjectionPendingThenDisabledWithoutDocument() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.CPUPartitionConfigController{
		V1Alpha1Mode: machineruntime.ModeMetal,
	}))

	// No machine config yet: the projection is pending, not disabled.
	ctest.AssertNoResource[*runtime.CPUPartitionSpec](suite, runtime.CPUPartitionSpecID)

	watchdog := runtimecfg.NewWatchdogTimerV1Alpha1()
	watchdog.WatchdogDevice = "/dev/watchdog0"

	cfg, err := container.New(watchdog)
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	// Machine config without the document: a confirmed absence of policy.
	ctest.AssertResource(suite, runtime.CPUPartitionSpecID, func(res *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
		asrt.Equal(runtime.CPUPartitionSpecSpec{}, *res.TypedSpec())
		asrt.Equal("runtime.CPUPartitionConfigController", res.Metadata().Owner())
	})

	// Introducing the document turns the policy on with canonical CPU lists.
	suite.replaceConfig(watchdog, newCPUPartitionConfig())

	ctest.AssertResource(suite, runtime.CPUPartitionSpecID, func(res *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
		asrt.Equal(acceptedSpec(), *res.TypedSpec())
	})

	// Removing it again confirms the absence rather than leaving the projection missing.
	suite.replaceConfig(watchdog)

	ctest.AssertResource(suite, runtime.CPUPartitionSpecID, func(res *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
		asrt.Equal(runtime.CPUPartitionSpecSpec{}, *res.TypedSpec())
	})

	// Removing the machine config removes the projection.
	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)
	suite.Destroy(old)

	ctest.AssertNoResource[*runtime.CPUPartitionSpec](suite, runtime.CPUPartitionSpecID)
}

// The intermediate config must be observed and fully reconciled before the version is checked:
// inside a synctest bubble, synctest.Wait returns only once every goroutine of the runtime is
// durably blocked, i.e. the controller has drained its event channel, so the ordering is
// config update -> reconcile finished -> version compared, with no timing threshold.
func TestCPUPartitionConfigUnrelatedChangeLeavesSpecUntouched(t *testing.T) {
	t.Parallel()

	synctest.Test(t, func(t *testing.T) {
		suite := &CPUPartitionConfigSuite{Timeout: 30 * time.Second}
		suite.SetT(t)
		suite.SetupTest()

		defer suite.TearDownTest()

		suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.CPUPartitionConfigController{
			V1Alpha1Mode: machineruntime.ModeMetal,
		}))

		watchdog := runtimecfg.NewWatchdogTimerV1Alpha1()
		watchdog.WatchdogDevice = "/dev/watchdog0"

		cfg, err := container.New(watchdog, newCPUPartitionConfig())
		suite.Require().NoError(err)
		suite.Create(config.NewMachineConfig(cfg))

		synctest.Wait()

		spec, err := safe.StateGetByID[*runtime.CPUPartitionSpec](suite.Ctx(), suite.State(), runtime.CPUPartitionSpecID)
		suite.Require().NoError(err)
		suite.Require().Equal(acceptedSpec(), *spec.TypedSpec())

		version := spec.Metadata().Version()

		// An unrelated document change and a non-canonical rewrite of the same policy must not
		// produce a new spec version: consumers would otherwise churn on every config apply.
		watchdog.WatchdogDevice = "/dev/watchdog1"

		rewritten := newCPUPartitionConfig()
		rewritten.InitConfig.RootCPUs = "0,1"
		rewritten.VirtualMachinesConfig.SlicesConfig[0].SliceCPUs = "4,5"

		suite.replaceConfig(watchdog, rewritten)

		synctest.Wait()

		suite.Assert().Equal(version, suite.specVersion(), "an unrelated config change must not rewrite the spec")

		// A real policy change does bump the version, proving the reconcile above did run.
		changed := newCPUPartitionConfig()
		changed.KubepodsConfig.RootCPUs = "2"
		suite.replaceConfig(watchdog, changed)

		synctest.Wait()

		spec, err = safe.StateGetByID[*runtime.CPUPartitionSpec](suite.Ctx(), suite.State(), runtime.CPUPartitionSpecID)
		suite.Require().NoError(err)
		suite.Assert().Equal("2", spec.TypedSpec().Roots["kubepods"])
		suite.Assert().Equal(version.Next(), spec.Metadata().Version(), "exactly one spec write is expected across three config versions")
	})
}

func (suite *CPUPartitionConfigSuite) TestContainerModeIsConfirmedNoPolicy() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimectrls.CPUPartitionConfigController{
		V1Alpha1Mode: machineruntime.ModeContainer,
	}))

	cfg, err := container.New(newCPUPartitionConfig())
	suite.Require().NoError(err)
	suite.Create(config.NewMachineConfig(cfg))

	// The document parses and validates, but in container mode the projected policy is
	// disabled: the spec is published (nothing waits on it) with no roots or slices.
	ctest.AssertResource(suite, runtime.CPUPartitionSpecID, func(res *runtime.CPUPartitionSpec, asrt *assert.Assertions) {
		asrt.Equal(runtime.CPUPartitionSpecSpec{}, *res.TypedSpec())
	})
}
