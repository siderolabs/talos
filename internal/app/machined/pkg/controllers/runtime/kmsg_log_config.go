// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/constants"
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
	return nil
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
//nolint:gocyclo,cyclop
func (ctrl *KmsgLogConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) (err error) {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		destinations := []*url.URL{}

		if ctrl.Cmdline != nil {
			if val := ctrl.Cmdline.Get(constants.KernelParamLoggingKernel).First(); val != nil {
				destURL, err := url.Parse(*val)
				if err != nil {
					return fmt.Errorf("error parsing %q: %w", constants.KernelParamLoggingKernel, err)
				}

				destinations = append(destinations, destURL)
			}
		}

		if len(destinations) == 0 {
			if err := r.Destroy(ctx, runtime.NewKmsgLogConfig().Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error destroying kmsg log config: %w", err)
			}
		} else {
			if err := safe.WriterModify(ctx, r, runtime.NewKmsgLogConfig(), func(cfg *runtime.KmsgLogConfig) error {
				cfg.TypedSpec().Destinations = destinations

				return nil
			}); err != nil {
				return fmt.Errorf("error updating kmsg log config: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}
