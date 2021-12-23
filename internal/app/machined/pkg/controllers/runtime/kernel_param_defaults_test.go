// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/go-retry/retry"

	runtimecontrollers "github.com/talos-systems/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/kernel"
	runtimeresource "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

type KernelParamDefaultsSuite struct {
	RuntimeSuite
}

func getParams(mode runtime.Mode) []*kernel.Param {
	res := []*kernel.Param{
		{
			Key:   "net.ipv4.ip_forward",
			Value: "1",
		},
		{
			Key:   "net.ipv6.conf.default.forwarding",
			Value: "1",
		},
		{
			Key:   "kernel.pid_max",
			Value: "262144",
		},
	}

	if mode != runtime.ModeContainer {
		res = append(res, []*kernel.Param{
			{
				Key:   "net.bridge.bridge-nf-call-iptables",
				Value: "1",
			},
			{
				Key:   "net.bridge.bridge-nf-call-ip6tables",
				Value: "1",
			},
		}...)
	}

	return res
}

//nolint:dupl
func (suite *KernelParamDefaultsSuite) TestContainerMode() {
	controller := &runtimecontrollers.KernelParamDefaultsController{
		runtime.ModeContainer,
	}

	suite.Require().NoError(suite.runtime.RegisterController(controller))

	suite.startRuntime()

	for _, prop := range getParams(runtime.ModeContainer) {
		prop := prop

		suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResource(
				resource.NewMetadata(runtimeresource.NamespaceName, runtimeresource.KernelParamDefaultSpecType, prop.Key, resource.VersionUndefined),
				func(res resource.Resource) bool {
					return res.(runtimeresource.KernelParam).TypedSpec().Value == prop.Value
				},
			),
		))
	}
}

//nolint:dupl
func (suite *KernelParamDefaultsSuite) TestMetalMode() {
	controller := &runtimecontrollers.KernelParamDefaultsController{
		runtime.ModeMetal,
	}

	suite.Require().NoError(suite.runtime.RegisterController(controller))

	suite.startRuntime()

	for _, prop := range getParams(runtime.ModeMetal) {
		prop := prop

		suite.Assert().NoError(retry.Constant(10*time.Second, retry.WithUnits(100*time.Millisecond)).Retry(
			suite.assertResource(
				resource.NewMetadata(runtimeresource.NamespaceName, runtimeresource.KernelParamDefaultSpecType, prop.Key, resource.VersionUndefined),
				func(res resource.Resource) bool {
					return res.(runtimeresource.KernelParam).TypedSpec().Value == prop.Value
				},
			),
		))
	}
}

func TestKernelParamDefaultsSuite(t *testing.T) {
	suite.Run(t, new(KernelParamDefaultsSuite))
}
