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
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	runtimeresource "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type KernelParamConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *KernelParamConfigSuite) TestReconcileConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelParamConfigController{}))

	const (
		value      = "500000"
		valueSysfs = "600000"
	)

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineSysctls: map[string]string{
						fsFileMax: value,
					},
					MachineSysfs: map[string]string{
						fsFileMax: valueSysfs,
					},
				},
				ClusterConfig: &v1alpha1.ClusterConfig{},
			},
		),
	)

	suite.Create(cfg)

	ctest.AssertResource(suite, procSysfsFileMax, func(r *runtimeresource.KernelParamSpec, asrt *assert.Assertions) {
		asrt.Equal(value, r.TypedSpec().Value)
	})

	ctest.AssertResource(suite, sysfsFileMax, func(r *runtimeresource.KernelParamSpec, asrt *assert.Assertions) {
		asrt.Equal(valueSysfs, r.TypedSpec().Value)
	})

	ctest.UpdateWithConflicts(suite, cfg, func(r *config.MachineConfig) error {
		r.Container().RawV1Alpha1().MachineConfig.MachineSysctls = map[string]string{}

		return nil
	})

	ctest.AssertNoResource[*runtimeresource.KernelParamSpec](suite, procSysfsFileMax)
	ctest.AssertResource(suite, sysfsFileMax, func(r *runtimeresource.KernelParamSpec, asrt *assert.Assertions) {
		asrt.Equal(valueSysfs, r.TypedSpec().Value)
	})
}

func TestKernelParamConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KernelParamConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
