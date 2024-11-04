// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"net/url"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// KmsgLogConfigController generates configuration for kmsg log delivery.
type KmsgLogConfigController struct {
	Cmdline *procfs.Cmdline
}

// Name implements controller.Controller interface.
func (ctrl *KmsgLogConfigController) Name() string {
	return "runtime.KmsgLogConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KmsgLogConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *KmsgLogConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.KmsgLogConfigType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *KmsgLogConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) (err error) {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		var destinations []*url.URL

		if ctrl.Cmdline != nil {
			if val := ctrl.Cmdline.Get(constants.KernelParamLoggingKernel).First(); val != nil {
				destURL, err := url.Parse(*val)
				if err != nil {
					return fmt.Errorf("error parsing %q: %w", constants.KernelParamLoggingKernel, err)
				}

				destinations = append(destinations, destURL)
			}
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting machine config: %w", err)
		}

		if cfg != nil {
			// remove duplicate URLs in case same destination is specified in both machine config and kernel args
			destinations = append(destinations, xslices.Filter(cfg.Config().Runtime().KmsgLogURLs(),
				func(u *url.URL) bool {
					return !slices.ContainsFunc(destinations, func(v *url.URL) bool {
						return v.String() == u.String()
					})
				})...)
		}

		r.StartTrackingOutputs()

		if len(destinations) > 0 {
			if err = safe.WriterModify(ctx, r, runtime.NewKmsgLogConfig(), func(cfg *runtime.KmsgLogConfig) error {
				cfg.TypedSpec().Destinations = destinations

				return nil
			}); err != nil {
				return fmt.Errorf("error updating kmsg log config: %w", err)
			}
		}

		if err = safe.CleanupOutputs[*runtime.KmsgLogConfig](ctx, r); err != nil {
			return err
		}
	}
}
