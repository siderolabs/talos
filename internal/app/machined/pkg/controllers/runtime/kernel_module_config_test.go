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

type KernelModuleConfigSuite struct {
	ctest.DefaultSuite
}

func (suite *KernelModuleConfigSuite) TestReconcileConfig() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.KernelModuleConfigController{}))

	cfg := config.NewMachineConfig(
		container.NewV1Alpha1(
			&v1alpha1.Config{
				ConfigVersion: "v1alpha1",
				MachineConfig: &v1alpha1.MachineConfig{
					MachineKernel: &v1alpha1.KernelConfig{
						KernelModules: []*v1alpha1.KernelModuleConfig{
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
		r.Container().RawV1Alpha1().MachineConfig.MachineKernel = nil

		return nil
	})

	for _, name := range []string{"btrfs", "e1000"} {
		ctest.AssertNoResource[*runtimeresource.KernelModuleSpec](suite, name)
	}
}

func TestKernelModuleConfigSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &KernelModuleConfigSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 5 * time.Second,
		},
	})
}
