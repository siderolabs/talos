// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// LogPersistenceController is a controller that persists logs in files.
type LogPersistenceController struct {
	V1Alpha1Logging runtime.LoggingManager

	// dummy implementation
	canLog atomic.Bool
}

// Name implements controller.Controller interface.
func (ctrl *LogPersistenceController) Name() string {
	return "runtime.LogPersistenceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *LogPersistenceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *LogPersistenceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

func (ctrl *LogPersistenceController) WriteLog(id string, line []byte) error {
	if !ctrl.canLog.Load() {
		// logging is not enabled, drop the log line
		return nil
	}

	f, err := os.OpenFile(filepath.Join(constants.LogMountPoint, id+".log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("error opening log file for %q: %w", id, err)
	}

	if _, err = f.Write(append(line, '\n')); err != nil {
		f.Close()

		return fmt.Errorf("error writing log line for %q: %w", id, err)
	}

	if err = f.Close(); err != nil {
		return fmt.Errorf("error closing log file for %q: %w", id, err)
	}

	return nil
}

func (ctrl *LogPersistenceController) startLogging() {
	// [TODO]: here we can start logging activities
	ctrl.canLog.Store(true)
}

func (ctrl *LogPersistenceController) stopLogging() {
	// [TODO]: here we should stop all logging activities, close files, flush buffers, etc.
	// after this call we should not hold /var/log
	ctrl.canLog.Store(false)
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *LogPersistenceController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	ctrl.V1Alpha1Logging.SetLineWriter(ctrl)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		requestID := ctrl.Name() + "-" + constants.LogMountPoint

		// create a volume mount request for the logs volume mount point
		// to keep it alive and prevent it from being torn down
		if err := safe.WriterModify(ctx, r,
			block.NewVolumeMountRequest(block.NamespaceName, requestID),
			func(v *block.VolumeMountRequest) error {
				v.TypedSpec().Requester = ctrl.Name()
				v.TypedSpec().VolumeID = constants.LogMountPoint

				return nil
			},
		); err != nil {
			return fmt.Errorf("error creating volume mount request for user volume mount point: %w", err)
		}

		vms, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, requestID)
		if err != nil {
			if state.IsNotFoundError(err) {
				// volume mount not ready yet, wait more
				continue
			}

			return fmt.Errorf("error getting volume mount status for log volume: %w", err)
		}

		switch vms.Metadata().Phase() {
		case resource.PhaseRunning:
			if !vms.Metadata().Finalizers().Has(ctrl.Name()) {
				if err = r.AddFinalizer(ctx, vms.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error adding finalizer to volume mount status for log volume: %w", err)
				}

				ctrl.startLogging()
			}
		case resource.PhaseTearingDown:
			if vms.Metadata().Finalizers().Has(ctrl.Name()) {
				ctrl.stopLogging()

				if err = r.RemoveFinalizer(ctx, vms.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer from volume mount status for log volume: %w", err)
				}
			}
		}
	}
}
