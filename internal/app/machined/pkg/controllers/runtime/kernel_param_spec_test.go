// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	runtimecontrollers "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/runtime"
	krnl "github.com/talos-systems/talos/pkg/kernel"
	"github.com/talos-systems/talos/pkg/machinery/kernel"
	runtimeresource "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

type KernelParamSpecSuite struct {
	RuntimeSuite
}

func (suite *KernelParamSpecSuite) TestParamsSynced() {
	suite.Require().NoError(suite.runtime.RegisterController(&runtimecontrollers.KernelParamSpecController{}))

	suite.startRuntime()

	value := "500000"
	def := ""

	spec := runtimeresource.NewKernelParamSpec(runtimeresource.NamespaceName, fsFileMax)
	spec.TypedSpec().Value = value

	suite.Require().NoError(suite.state.Create(suite.ctx, spec))

	statusMD := resource.NewMetadata(runtimeresource.NamespaceName, runtimeresource.KernelParamStatusType, fsFileMax, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			statusMD,
			func(res resource.Resource) bool {
				def = res.(*runtimeresource.KernelParamStatus).TypedSpec().Default

				return res.(*runtimeresource.KernelParamStatus).TypedSpec().Current == value
			},
		),
	))

	prop, err := krnl.ReadParam(&kernel.Param{Key: fsFileMax})
	suite.Assert().NoError(err)
	suite.Require().Equal(value, strings.TrimSpace(string(prop)))

	suite.Require().NoError(suite.state.Destroy(suite.ctx, spec.Metadata()))

	// wait for the resource to be removed
	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		func() error {
			for _, md := range []resource.Metadata{statusMD} {
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

	prop, err = krnl.ReadParam(&kernel.Param{Key: fsFileMax})
	suite.Assert().NoError(err)
	suite.Require().Equal(def, strings.TrimSpace(string(prop)))
}

func (suite *KernelParamSpecSuite) TestParamsUnsupported() {
	suite.Require().NoError(suite.runtime.RegisterController(&runtimecontrollers.KernelParamSpecController{}))

	suite.startRuntime()

	id := "some.really.not.existing.sysctl"

	spec := runtimeresource.NewKernelParamSpec(runtimeresource.NamespaceName, id)
	spec.TypedSpec().Value = "value"
	spec.TypedSpec().IgnoreErrors = true

	suite.Require().NoError(suite.state.Create(suite.ctx, spec))

	statusMD := resource.NewMetadata(runtimeresource.NamespaceName, runtimeresource.KernelParamStatusType, id, resource.VersionUndefined)

	suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
		suite.assertResource(
			statusMD,
			func(res resource.Resource) bool {
				return res.(*runtimeresource.KernelParamStatus).TypedSpec().Unsupported == true
			},
		),
	))
}

func TestKernelParamSpecSuite(t *testing.T) {
	suite.Run(t, new(KernelParamSpecSuite))
}
