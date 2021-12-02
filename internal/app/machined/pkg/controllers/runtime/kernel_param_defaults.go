// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/kernel/kspp"
	"github.com/talos-systems/talos/pkg/machinery/kernel"
	"github.com/talos-systems/talos/pkg/machinery/resources/runtime"
)

// KernelParamDefaultsController creates default kernel params.
type KernelParamDefaultsController struct {
	V1Alpha1Mode v1alpha1runtime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *KernelParamDefaultsController) Name() string {
	return "runtime.KernelParamDefaultsController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KernelParamDefaultsController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *KernelParamDefaultsController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.KernelParamDefaultSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *KernelParamDefaultsController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
		kernelParams := ctrl.getKernelParams()
		if ctrl.V1Alpha1Mode != v1alpha1runtime.ModeContainer {
			kernelParams = append(kernelParams, kspp.GetKernelParams()...)
		}

		for _, prop := range kernelParams {
			value := prop.Value
			item := runtime.NewKernelParamDefaultSpec(runtime.NamespaceName, prop.Key)

			if err := r.Modify(ctx, item, func(res resource.Resource) error {
				res.(*runtime.KernelParamDefaultSpec).TypedSpec().Value = value

				if item.Metadata().ID() == "net.ipv6.conf.default.forwarding" {
					res.(*runtime.KernelParamDefaultSpec).TypedSpec().IgnoreErrors = true
				}

				return nil
			}); err != nil {
				return err
			}
		}
	}

	return nil
}

func (ctrl *KernelParamDefaultsController) getKernelParams() []*kernel.Param {
	res := []*kernel.Param{
		{
			Key:   "net.ipv4.ip_forward",
			Value: "1",
		},
	}

	if ctrl.V1Alpha1Mode != v1alpha1runtime.ModeContainer {
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

	res = append(res, []*kernel.Param{
		{
			Key:   "net.ipv6.conf.default.forwarding",
			Value: "1",
		},
		{
			Key:   "kernel.pid_max",
			Value: "262144",
		},
	}...)

	return res
}
