// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-procfs/procfs"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// UdevServiceManager is the interface to the v1alpha1 service subsystem.
type UdevServiceManager interface {
	IsRunning(id string) (system.Service, bool, error)
	Load(services ...system.Service) []string
	Start(serviceIDs ...string) error
}

// UdevServiceController owns udevd service startup.
type UdevServiceController struct {
	V1Alpha1Mode machineruntime.Mode

	V1Alpha1Services UdevServiceManager
	WaitForUdevd     func(ctx context.Context, serviceID string) error

	started bool
}

// Name implements controller.Controller interface.
func (ctrl *UdevServiceController) Name() string {
	return "runtime.UdevServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UdevServiceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtimeres.NamespaceName,
			Type:      runtimeres.KernelCmdlineType,
			ID:        optional.Some(runtimeres.KernelCmdlineID),
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UdevServiceController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *UdevServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.ensureStarted(ctx, r, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *UdevServiceController) ensureStarted(ctx context.Context, r controller.Reader, logger *zap.Logger) error {
	if ctrl.started {
		return nil
	}

	if ctrl.V1Alpha1Services == nil {
		return fmt.Errorf("udev service manager is not configured")
	}

	extraSettleTime, kernelCmdlineReady, err := ctrl.extraSettleTime(ctx, r, logger)
	if err != nil {
		return err
	}

	if !kernelCmdlineReady {
		return nil
	}

	service := &services.Udevd{
		ExtraSettleTime: extraSettleTime,
	}
	serviceID := service.ID(nil)

	ctrl.V1Alpha1Services.Load(service)

	_, running, err := ctrl.V1Alpha1Services.IsRunning(serviceID)
	if err != nil {
		return fmt.Errorf("failed to check udevd service state: %w", err)
	}

	if !running {
		if err = ctrl.V1Alpha1Services.Start(serviceID); err != nil {
			return fmt.Errorf("failed to start udevd service: %w", err)
		}
	}

	waitForUdevd := ctrl.WaitForUdevd
	if waitForUdevd == nil {
		waitForUdevd = func(ctx context.Context, serviceID string) error {
			waitCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
			defer cancel()

			return system.WaitForService(system.StateEventUp, serviceID).Wait(waitCtx)
		}
	}

	if err = waitForUdevd(ctx, serviceID); err != nil {
		return fmt.Errorf("failed waiting for udevd service: %w", err)
	}

	ctrl.started = true

	return nil
}

func (ctrl *UdevServiceController) extraSettleTime(ctx context.Context, r controller.Reader, logger *zap.Logger) (time.Duration, bool, error) {
	kernelCmdline, err := safe.ReaderGetByID[*runtimeres.KernelCmdline](ctx, r, runtimeres.KernelCmdlineID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return 0, false, nil
		}

		return 0, false, fmt.Errorf("failed to get kernel cmdline: %w", err)
	}

	return extraSettleTimeFromCmdline(kernelCmdline.TypedSpec().Cmdline, logger), true, nil
}

func extraSettleTimeFromCmdline(cmdline string, logger *zap.Logger) time.Duration {
	settleTimeStr := procfs.NewCmdline(cmdline).Get(constants.KernelParamDeviceSettleTime).First()
	if settleTimeStr == nil {
		return 0
	}

	extraSettleTime, err := time.ParseDuration(*settleTimeStr)
	if err != nil {
		logger.Warn("failed to parse extra udev settle time", zap.String("param", constants.KernelParamDeviceSettleTime), zap.Error(err))

		return 0
	}

	logger.Info("extra udev settle time", zap.Duration("duration", extraSettleTime))

	return extraSettleTime
}

var _ controller.Controller = &UdevServiceController{}
