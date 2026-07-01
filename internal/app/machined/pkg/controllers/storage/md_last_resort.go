// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/md"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// mdLastResortGracePeriod is how long an array may stay inactive (waiting for a
// missing member) after udevd has settled before it is force-started degraded.
// udevd's own settle already waits, so this is an additional margin to let a
// genuinely slow member appear before we start the array one-legged. Mirrors
// the intent of systemd's mdadm-last-resort@.timer, which Talos lacks.
const mdLastResortGracePeriod = 30 * time.Second

// MDLastResortController force-starts degraded MD arrays that udev's
// incremental assembly left inactive because a member is missing.
//
// Healthy arrays are assembled automatically by udev rules; this controller
// only handles the degraded case, which on systemd is covered by
// mdadm-last-resort@.timer. Without it, a RAID1 whose mirror disk failed would
// never start and the node could not boot from the survivor.
type MDLastResortController struct {
	V1Alpha1Mode machineruntime.Mode
	// GracePeriod overrides mdLastResortGracePeriod; zero uses the default.
	GracePeriod time.Duration

	mdOnce sync.Once
	md     *md.MD
	mdErr  error
}

// Name implements controller.Controller interface.
func (ctrl *MDLastResortController) Name() string {
	return "storage.MDLastResortController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MDLastResortController) Inputs() []controller.Input {
	return []controller.Input{
		{
			// Re-evaluate when block devices change: arrays appear inactive, a
			// late member shows up, or a force-run flips one to active.
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

// Outputs implements controller.Controller interface.
func (ctrl *MDLastResortController) Outputs() []controller.Output {
	return nil
}

// mdInstance lazily initializes the mdadm wrapper, caching any initialization
// error so it is only paid once.
func (ctrl *MDLastResortController) mdInstance() (*md.MD, error) {
	ctrl.mdOnce.Do(func() {
		ctrl.md, ctrl.mdErr = md.New()
	})

	return ctrl.md, ctrl.mdErr
}

// udevdReady reports whether udevd has finished enumerating block devices, so
// that udev's incremental assembly has already had its chance before we treat
// a still-inactive array as genuinely degraded.
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

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *MDLastResortController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode.IsAgent() {
		// in agent mode there is no local disk to assemble arrays from.
		return nil
	}

	grace := ctrl.GracePeriod
	if grace == 0 {
		grace = mdLastResortGracePeriod
	}

	// graceCh is armed when inactive arrays are first observed and disarmed
	// when it fires; force-run happens only after the grace elapses so a slow
	// member can still complete the array cleanly.
	var graceCh <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-graceCh:
			graceCh = nil

			mdInst, err := ctrl.mdInstance()
			if err != nil {
				return fmt.Errorf("failed to initialize mdadm: %w", err)
			}

			if err := ctrl.forceRunInactive(ctx, mdInst, logger); err != nil {
				// Do not fail the controller: a stubborn array must not wedge
				// machined. It will be retried if a later event re-arms grace.
				logger.Warn("failed to force-run degraded MD arrays", zap.Error(err))
			}

			continue
		}

		ready, err := ctrl.udevdReady(ctx, r, logger)
		if err != nil {
			return err
		}

		if !ready {
			continue
		}

		inactive, err := md.InactiveArrays()
		if err != nil {
			logger.Warn("failed to list MD arrays", zap.Error(err))

			continue
		}

		if len(inactive) > 0 && graceCh == nil {
			logger.Info(
				"inactive MD arrays detected; will force-run degraded after grace if still stopped",
				zap.Strings("arrays", inactive),
				zap.Duration("grace", grace),
			)

			graceCh = time.After(grace)
		}
	}
}

// forceRunInactive re-checks for inactive arrays (a member may have appeared
// during the grace window) and force-starts whatever remains stopped.
func (ctrl *MDLastResortController) forceRunInactive(ctx context.Context, mdInst *md.MD, logger *zap.Logger) error {
	inactive, err := md.InactiveArrays()
	if err != nil {
		return fmt.Errorf("failed to list MD arrays: %w", err)
	}

	var multiErr error

	for _, dev := range inactive {
		logger.Info("force-running degraded MD array", zap.String("device", dev))

		if err := mdInst.RunArray(ctx, dev); err != nil {
			multiErr = errors.Join(multiErr, fmt.Errorf("force-run %s: %w", dev, err))
		}
	}

	return multiErr
}
