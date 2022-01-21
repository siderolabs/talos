// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.
package runtime_test

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	runtimecontrollers "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/talos-systems/talos/pkg/machinery/kernel"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	runtimeresource "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

type KernelParamConfigSuite struct {
	RuntimeSuite
}

func (suite *KernelParamConfigSuite) TestReconcileConfig() {
	suite.Require().NoError(suite.runtime.RegisterController(&runtimecontrollers.KernelParamConfigController{}))

	suite.startRuntime()

	value := "500000"
	valueSysfs := "600000"

	cfg := config.NewMachineConfig(&v1alpha1.Config{
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
	})
	key := filepath.Join(kernel.Sysctl, strings.ReplaceAll(fsFileMax, ".", "/"))

	suite.Require().NoError(suite.state.Create(suite.ctx, cfg))

	sysctlMD := resource.NewMetadata(runtimeresource.NamespaceName, runtimeresource.KernelParamSpecType, "sysctl/"+fsFileMax, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			sysctlMD,
			func(res resource.Resource) bool {
				spec := res.(*runtimeresource.KernelParamSpec).TypedSpec()

				return suite.Assert().Equal(value, spec.Value) && suite.Assert().Equal(key, spec.Key)
			},
		),
	))

	sysfsMD := resource.NewMetadata(runtimeresource.NamespaceName, runtimeresource.KernelParamSpecType, "sysfs/"+fsFileMax, resource.VersionUndefined)
	key = filepath.Join(kernel.Sysfs, strings.ReplaceAll(fsFileMax, ".", "/"))

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			sysfsMD,
			func(res resource.Resource) bool {
				spec := res.(*runtimeresource.KernelParamSpec).TypedSpec()

				return suite.Assert().Equal(valueSysfs, spec.Value) && suite.Assert().Equal(key, spec.Key)
			},
		),
	))

	cfg = config.NewMachineConfig(&v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineSysctls: map[string]string{},
			MachineSysfs: map[string]string{
				fsFileMax: valueSysfs,
			},
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
			for _, md := range []resource.Metadata{sysctlMD} {
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

func TestKernelParamConfigSuite(t *testing.T) {
	suite.Run(t, new(KernelParamConfigSuite))
}
