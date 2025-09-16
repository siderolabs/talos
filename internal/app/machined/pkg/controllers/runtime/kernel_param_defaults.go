// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"os"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/kernel/kspp"
	"github.com/siderolabs/talos/pkg/machinery/kernel"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
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
func (ctrl *KernelParamDefaultsController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
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

			if err := safe.WriterModify(ctx, r, item, func(res *runtime.KernelParamDefaultSpec) error {
				res.TypedSpec().Value = value

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
			Key:   "proc.sys.net.ipv4.ip_forward",
			Value: "1",
		},
		{
			Key:   "proc.sys.net.ipv4.icmp_ignore_bogus_error_responses",
			Value: "1",
		},
		{
			Key:   "proc.sys.net.ipv4.icmp_echo_ignore_broadcasts",
			Value: "1",
		},
	}

	if ctrl.V1Alpha1Mode != v1alpha1runtime.ModeContainer {
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

	// Apply IPv6 defaults only if IPv6 is enabled.
	// NB: we only prevent the application of these rules if the IPv6 node does not exist.
	// Other errors should be ignored here so that they bubble up later, where errors can be logged and handled.
	_, err := os.Stat("/proc/sys/net/ipv6/conf/default/accept_ra")
	if err == nil || !errors.Is(err, os.ErrNotExist) {
		res = append(res, []*kernel.Param{
			{
				Key:   "proc.sys.net.ipv6.conf.default.forwarding",
				Value: "1",
			},
			{
				Key:   "proc.sys.net.ipv6.conf.default.accept_ra",
				Value: "2",
			},
		}...)
	}

	res = append(res, []*kernel.Param{
		// ipvs/conntrack tcp keepalive refresh.
		{
			Key:   "proc.sys.net.ipv4.tcp_keepalive_time",
			Value: "600",
		},
		{
			Key:   "proc.sys.net.ipv4.tcp_keepalive_intvl",
			Value: "60",
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
	}...)

	// kernel optimization for kubernetes workloads.
	res = append(res, []*kernel.Param{
		// configs inotify.
		{
			Key:   "proc.sys.fs.inotify.max_user_instances",
			Value: "8192",
		},
		{
			Key:   "proc.sys.fs.aio-max-nr",
			Value: "1048576",
		},
	}...)

	return res
}
