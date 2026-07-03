// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const mdLastResortGracePeriod = 30 * time.Second

// MDLastResortBackend lists and force-runs inactive MD arrays.
type MDLastResortBackend interface {
	InactiveArrays() ([]string, error)
	RunArray(ctx context.Context, device string) error
}

// MDLastResortController force-starts degraded MD arrays left inactive by udev.
type MDLastResortController struct {
	V1Alpha1Mode machineruntime.Mode
	GracePeriod  time.Duration
	MD           MDLastResortBackend
}

// Name implements controller.Controller.
func (ctrl *MDLastResortController) Name() string {
	return "storage.MDLastResortController"
}

// Inputs implements controller.Controller.
func (ctrl *MDLastResortController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.DeviceType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller.
func (ctrl *MDLastResortController) Outputs() []controller.Output {
	return nil
}

func (ctrl *MDLastResortController) udevdReady(ctx context.Context, r controller.Reader, logger *zap.Logger) (bool, error) {
	udevdService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "udevd")
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("failed to get udevd service: %w", err)
	}

	if udevdService == nil || !udevdService.TypedSpec().Running || !udevdService.TypedSpec().Healthy {
		logger.Debug("waiting for udevd service to be running and healthy")

		return false, nil
	}

	return true, nil
}

// Run implements controller.Controller.
func (ctrl *MDLastResortController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.IsAgent() {
		return nil
	}

	grace := ctrl.gracePeriod()

	var graceCh <-chan time.Time

	for {
		var fired bool

		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-graceCh:
			graceCh = nil
			fired = true
		}

		if fired {
			if err := ctrl.forceRunInactive(ctx, logger); err != nil {
				logger.Warn("failed to force-run degraded MD arrays", zap.Error(err))
			}

			continue
		}

		var err error

		graceCh, err = ctrl.handleEvent(ctx, r, logger, grace, graceCh)
		if err != nil {
			return err
		}
	}
}

func (ctrl *MDLastResortController) gracePeriod() time.Duration {
	if ctrl.GracePeriod != 0 {
		return ctrl.GracePeriod
	}

	return mdLastResortGracePeriod
}

func (ctrl *MDLastResortController) handleEvent(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	grace time.Duration,
	graceCh <-chan time.Time,
) (<-chan time.Time, error) {
	ready, err := ctrl.udevdReady(ctx, r, logger)
	if err != nil {
		return graceCh, err
	}

	if !ready {
		return graceCh, nil
	}

	return ctrl.armGraceIfInactive(logger, grace, graceCh), nil
}

func (ctrl *MDLastResortController) armGraceIfInactive(logger *zap.Logger, grace time.Duration, graceCh <-chan time.Time) <-chan time.Time {
	if graceCh != nil {
		return graceCh
	}

	inactive, err := ctrl.MD.InactiveArrays()
	if err != nil {
		logger.Warn("failed to list MD arrays", zap.Error(err))

		return nil
	}

	if len(inactive) == 0 {
		return nil
	}

	logger.Info("inactive MD arrays detected; will force-run degraded after grace if still stopped", zap.Strings("arrays", inactive), zap.Duration("grace", grace))

	return time.After(grace)
}

func (ctrl *MDLastResortController) forceRunInactive(ctx context.Context, logger *zap.Logger) error {
	inactive, err := ctrl.MD.InactiveArrays()
	if err != nil {
		return fmt.Errorf("failed to list MD arrays: %w", err)
	}

	var multiErr error

	for _, dev := range inactive {
		logger.Info("force-running degraded MD array", zap.String("device", dev))

		if err := ctrl.MD.RunArray(ctx, dev); err != nil {
			multiErr = errors.Join(multiErr, fmt.Errorf("force-run %s: %w", dev, err))
		}
	}

	return multiErr
}
