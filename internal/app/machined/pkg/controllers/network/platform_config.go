// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// PlatformConfigController runs the platform config acquire code and publishes the result as a resource.
type PlatformConfigController struct {
	V1alpha1Platform v1alpha1runtime.Platform
	PlatformState    state.State
}

// Name implements controller.Controller interface.
func (ctrl *PlatformConfigController) Name() string {
	return "network.PlatformConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PlatformConfigController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *PlatformConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.PlatformConfigType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *PlatformConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	if ctrl.V1alpha1Platform == nil {
		// no platform, no work to be done
		return nil
	}

	platformCtx, platformCtxCancel := context.WithCancel(ctx)
	defer platformCtxCancel()

	platformCh := make(chan *v1alpha1runtime.PlatformNetworkConfig, 1)

	var platformWg sync.WaitGroup

	platformWg.Add(1)

	go func() {
		defer platformWg.Done()

		ctrl.runWithRestarts(platformCtx, logger, func() error {
			return ctrl.V1alpha1Platform.NetworkConfiguration(platformCtx, ctrl.PlatformState, platformCh)
		})
	}()

	defer platformWg.Wait()

	r.QueueReconcile()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case networkConfig := <-platformCh:
			if networkConfig == nil {
				continue
			}

			if err := safe.WriterModify(ctx, r,
				network.NewPlatformConfig(network.NamespaceName, network.PlatformConfigActiveID),
				func(out *network.PlatformConfig) error {
					*out.TypedSpec() = *networkConfig

					return nil
				},
			); err != nil {
				return fmt.Errorf("error modifying active network config: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *PlatformConfigController) runWithRestarts(ctx context.Context, logger *zap.Logger, f func() error) {
	backoff := backoff.NewExponentialBackOff()

	// disable number of retries limit
	backoff.MaxElapsedTime = 0

	for ctx.Err() == nil {
		var err error
		if err = ctrl.runWithPanicHandler(logger, f); err == nil {
			// operator finished without an error
			return
		}

		// skip restarting if context is already done
		select {
		case <-ctx.Done():
			return
		default:
		}

		interval := backoff.NextBackOff()

		logger.Error("restarting platform network config", zap.Duration("interval", interval), zap.Error(err))

		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}

func (ctrl *PlatformConfigController) runWithPanicHandler(logger *zap.Logger, f func() error) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("panic: %v", p)

			logger.Error("platform panicked", zap.Stack("stack"), zap.Error(err))
		}
	}()

	err = f()

	return
}
