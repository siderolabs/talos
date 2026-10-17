// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package runtime_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	runtimeconfig "github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	runtimeresource "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

type KernelModuleConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *KernelModuleConfigSuite) TestReconcileLegacyConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelModuleConfigController{}))

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKernel: &v1alpha1.KernelConfig{ //nolint:staticcheck // legacy configuration
						KernelModules: []*v1alpha1.KernelModuleConfig{ //nolint:staticcheck // legacy configuration
							{
								ModuleName: "btrfs",
							},
							{
								ModuleName: "e1000",
							},
						},
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Create(cfg)

	for _, name := range []string{"btrfs", "e1000"} {
		ctest.AssertResource(suite, name, func(r *runtimeresource.KernelModuleSpec, asrt *assert.Assertions) {
			asrt.Equal(name, r.TypedSpec().Name)
		})
	}

	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		r.Container().RawV1Alpha1().MachineConfig.MachineKernel = nil //nolint:staticcheck // legacy configuration

		return nil
	})

	for _, name := range []string{"btrfs", "e1000"} {
		ctest.AssertNoResource[*runtimeresource.KernelModuleSpec](suite, name)
	}
}

func (suite *KernelModuleConfigSuite) TestReconcileMultiDocConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelModuleConfigController{}))

	overlapping := runtimeconfig.NewKernelModuleConfigV1Alpha1("btrfs")
	overlapping.ModuleParameters = []string{"multi-doc-param"}

	standalone := runtimeconfig.NewKernelModuleConfigV1Alpha1("vrf")

	ctr, err := container.New(
		&v1alpha1.Config{
			ConfigVersion: "v1alpha1",
			MachineConfig: &v1alpha1.MachineConfig{
				MachineKernel: &v1alpha1.KernelConfig{ //nolint:staticcheck // legacy configuration
					KernelModules: []*v1alpha1.KernelModuleConfig{ //nolint:staticcheck // legacy configuration
						{
							ModuleName:       "btrfs",
							ModuleParameters: []string{"legacy-v1alpha1-param"},
						},
					},
				},
			},
			ClusterConfig: &v1alpha1.ClusterConfig{},
		},
		overlapping,
		standalone,
	)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)

	suite.Create(cfg)

	// Module name conflict, multi-doc wins (including the position)
	ctest.AssertResource(suite, "btrfs", func(r *runtimeresource.KernelModuleSpec, asrt *assert.Assertions) {
		asrt.Equal([]string{"multi-doc-param"}, r.TypedSpec().Parameters)
		asrt.Equal(2, r.TypedSpec().Order)
	})

	// No conflict
	ctest.AssertResource(suite, "vrf", func(r *runtimeresource.KernelModuleSpec, asrt *assert.Assertions) {
		asrt.Equal("vrf", r.TypedSpec().Name)
		asrt.Equal(3, r.TypedSpec().Order)
	})
}

func (suite *KernelModuleConfigSuite) TestReconcileOrder() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelModuleConfigController{}))

	// A driver which requests its sub-drivers while probing (earth-pt3 -> tc90522, qm1d1c0042)
	// has to be loaded last, although its name sorts first.
	names := []string{"tc90522", "qm1d1c0042", "earth-pt3"}

	docs := make([]configconfig.Document, 0, len(names))

	for _, name := range names {
		docs = append(docs, runtimeconfig.NewKernelModuleConfigV1Alpha1(name))
	}

	ctr, err := container.New(docs...)
	suite.Require().NoError(err)

	suite.Create(config.NewMachineConfig(ctr))

	for i, name := range names {
		ctest.AssertResource(suite, name, func(r *runtimeresource.KernelModuleSpec, asrt *assert.Assertions) {
			asrt.Equal(i+1, r.TypedSpec().Order)
		})
	}
}

func (suite *KernelModuleConfigSuite) TestReconcileConfiguredAndRequested() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelModuleConfigController{}))

	md0 := storage.NewMDArraySpec(storage.NamespaceName, "md0")
	md0.TypedSpec().Level = storage.MDLevelRAID1

	suite.Create(md0)

	raid1 := runtimeconfig.NewKernelModuleConfigV1Alpha1("raid1")
	raid1.ModuleParameters = []string{"param"}

	ctr, err := container.New(runtimeconfig.NewKernelModuleConfigV1Alpha1("btrfs"), raid1)
	suite.Require().NoError(err)

	cfg := config.NewMachineConfig(ctr)

	suite.Create(cfg)

	// listed in the machine config and requested by Talos: the machine config wins
	ctest.AssertResource(suite, "raid1", func(r *runtimeresource.KernelModuleSpec, asrt *assert.Assertions) {
		asrt.Equal([]string{"param"}, r.TypedSpec().Parameters)
		asrt.Equal(2, r.TypedSpec().Order)
	})

	suite.Destroy(cfg)

	// still requested by Talos, nothing left from the machine config
	ctest.AssertResource(suite, "raid1", func(r *runtimeresource.KernelModuleSpec, asrt *assert.Assertions) {
		asrt.Empty(r.TypedSpec().Parameters)
		asrt.Zero(r.TypedSpec().Order)
	})
}

func (suite *KernelModuleConfigSuite) TestReconcileMDArraySpecs() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelModuleConfigController{}))

	md0 := storage.NewMDArraySpec(storage.NamespaceName, "md0")
	md0.TypedSpec().Level = storage.MDLevelRAID1

	md1 := storage.NewMDArraySpec(storage.NamespaceName, "md1")
	md1.TypedSpec().Level = storage.MDLevelRAID1

	suite.Create(md0)
	suite.Create(md1)

	// both arrays collapse into a single raid1 module spec
	ctest.AssertResource(suite, "raid1", func(r *runtimeresource.KernelModuleSpec, asrt *assert.Assertions) {
		asrt.Equal("raid1", r.TypedSpec().Name)
		asrt.Empty(r.TypedSpec().Parameters)
	})

	// one array left, module is still required
	suite.Destroy(md0)

	ctest.AssertResource(suite, "raid1", func(r *runtimeresource.KernelModuleSpec, asrt *assert.Assertions) {
		asrt.Equal("raid1", r.TypedSpec().Name)
	})

	suite.Destroy(md1)

	ctest.AssertNoResource[*runtimeresource.KernelModuleSpec](suite, "raid1")
}

func TestKernelModuleConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KernelModuleConfigSuite{
		Timeout: 5 * time.Second,
	})
}
