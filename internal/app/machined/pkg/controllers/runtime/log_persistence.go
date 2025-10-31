// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/concurrent"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime/internal/logfile"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// LogPersistenceController is a controller that persists logs in files.
type LogPersistenceController struct {
	V1Alpha1Logging runtime.LoggingManager

	startup sync.Once
	// RLocked by the log writers, Locked by volume handlers
	canLog        sync.RWMutex
	files         *concurrent.HashTrieMap[string, *logfile.LogFile]
	logMountPoint string
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

// WriteLog writes a single log line into the corresponding file.
func (ctrl *LogPersistenceController) WriteLog(id string, line []byte) error {
	ctrl.canLog.RLock()
	defer ctrl.canLog.RUnlock()

	lf, _ := ctrl.files.LoadOrStore(
		id,
		logfile.NewLogFile(
			filepath.Join(ctrl.logMountPoint, id+".log"),
			constants.LogRotateThreshold,
		),
	)

	return lf.Write(line)
}

func (ctrl *LogPersistenceController) startLogging(vms *block.VolumeMountStatus) {
	// here we can start logging activities
	ctrl.logMountPoint = vms.TypedSpec().Target

	ctrl.canLog.Unlock()
}

func (ctrl *LogPersistenceController) stopLogging() error {
	// Stop all logging activities, close files
	// after this call we should not hold /var/log
	ctrl.canLog.Lock()

	for _, f := range ctrl.files.All() {
		if err := f.Close(); err != nil {
			return fmt.Errorf("failed to close log buffer %w", err)
		}
	}

	ctrl.files.Clear()

	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *LogPersistenceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ctrl.startup.Do(func() {
		ctrl.files = concurrent.NewHashTrieMap[string, *logfile.LogFile]()
		// Block writes until /var/log is ready
		ctrl.canLog.Lock()

		ctrl.V1Alpha1Logging.SetLineWriter(ctrl)
	})

	ticker := time.NewTicker(constants.LogFlushPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			for _, f := range ctrl.files.All() {
				if err := f.Flush(); err != nil {
					return fmt.Errorf("failed to flush log buffer %w", err)
				}
			}

			continue
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

				ctrl.startLogging(vms)
			}
		case resource.PhaseTearingDown:
			if vms.Metadata().Finalizers().Has(ctrl.Name()) {
				if err = ctrl.stopLogging(); err != nil {
					return fmt.Errorf("error stopping persistent logging: %w", err)
				}

				if err = r.RemoveFinalizer(ctx, vms.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer from volume mount status for log volume: %w", err)
				}
			}
		}
	}
}
