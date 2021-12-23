// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package runtime_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	runtimecontrollers "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	runtimeresource "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

type KernelModuleConfigSuite struct {
	RuntimeSuite
}

func (suite *KernelModuleConfigSuite) TestReconcileConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&runtimecontrollers.KernelModuleConfigController{}))

	suite.startRuntime()

	cfg := config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineKernel: &v1alpha1.KernelConfig{
				KernelModules: []*v1alpha1.KernelModuleConfig{
					{
						ModuleName: "brtfs",
					},
					{
						ModuleName: "e1000",
					},
				},
			},
		},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	})

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	specMD := resource.NewMetadata(runtimeresource.NamespaceName, runtimeresource.KernelModuleSpecType, "e1000", resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			specMD,
			func(res resource.Resource) bool {
				return res.(*runtimeresource.KernelModuleSpec).TypedSpec().Name == "e1000"
			},
		),
	))

	cfg = config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineKernel: nil,
		},
		ClusterConfig: &v1alpha1.ClusterConfig{},
	})

	old := cfg.Metadata().Version()

	cfg.Metadata().BumpVersion()

	suite.Require().NoError(suite.state.Update(suite.ctx, old, cfg))

	var err error

	// wait for the resource to be removed
	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			for _, md := range []resource.Metadata{specMD} {
				_, err = suite.state.Get(suite.ctx, md)
				if err != nil {
					if state.IsNotFoundError(err) {
						return nil
					}

					return err
				}
			}

			return retry.ExpectedError(fmt.Errorf("resource still exists"))
		},
	))
}

func TestKernelModuleConfigSuite(t *testing.T) {
	suite.Run(t, new(KernelModuleConfigSuite))
}
