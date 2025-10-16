// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/suite"

	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	runtimeresource "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type KernelParamDefaultsSuite struct {
	RuntimeSuite
}

func getParams(mode runtime.Mode) []*kernel.Param {
	res := []*kernel.Param{
		{
			Key:   "proc.sys.net.ipv4.ip_forward",
			Value: "1",
		},
		{
			Key:   "proc.sys.net.ipv6.conf.default.forwarding",
			Value: "1",
		},
		{
			Key:   "proc.sys.net.ipv6.conf.default.accept_ra",
			Value: "2",
		},
		{
			Key:   "proc.sys.kernel.panic",
			Value: "10",
		},
		{
			Key:   "proc.sys.kernel.pid_max",
			Value: "262144",
		},
		{
			Key:   "proc.sys.vm.overcommit_memory",
			Value: "1",
		},
		{
			Key:   "proc.sys.net.ipv4.ip_local_reserved_ports",
			Value: "50000,50001",
		},
	}

	if mode != runtime.ModeContainer {
		res = append(res, []*kernel.Param{
			{
				Key:   "proc.sys.net.bridge.bridge-nf-call-iptables",
				Value: "1",
			},
			{
				Key:   "proc.sys.net.bridge.bridge-nf-call-ip6tables",
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
