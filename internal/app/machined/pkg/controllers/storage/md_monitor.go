// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/md"
	"github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// mdMonitorRestartBackoff throttles restarts of `mdadm --monitor`. Without it, a
// node with no MD arrays (mdadm exits immediately) spins the restart loop and
// floods the kernel log.
const mdMonitorRestartBackoff = 30 * time.Second

// MDMonitor is the mdadm monitor subset used by MDMonitorController.
type MDMonitor interface {
	Monitor(ctx context.Context, onEvent func(string)) error
}

// MDMonitorController runs mdadm monitor and emits MD refresh requests for each event.
type MDMonitorController struct {
	V1Alpha1Mode machineruntime.Mode
	MD           MDMonitor
}

// Name implements controller.Controller.
func (ctrl *MDMonitorController) Name() string {
	return "storage.MDMonitorController"
}

// Inputs implements controller.Controller.
func (ctrl *MDMonitorController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: storage.NamespaceName,
			Type:      storage.MDArraySpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: storage.NamespaceName,
			Type:      storage.MDArrayStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller.
func (ctrl *MDMonitorController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: storage.MDRefreshRequestType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller.
func (ctrl *MDMonitorController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	for {
		// Only (re)start mdadm on a storage change, with a backoff cap so a
		// monitor that dies while arrays are present still gets restarted even
		// without a fresh event. Gating on the event (rather than looping) keeps
		// a fast-exiting mdadm (e.g. no arrays present) from busy-looping and
		// flooding the kernel log.
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-time.After(mdMonitorRestartBackoff):
		}

		switch err := ctrl.runMonitor(ctx, r, logger); {
		case err == nil:
		case errors.Is(err, context.Canceled):
			return nil
		case errors.Is(err, md.ErrNotFound):
			logger.Debug("MD monitor found no redundant arrays, waiting for storage changes")
		default:
			logger.Warn("MD monitor exited, restarting", zap.Error(err))
		}
	}
}

func (ctrl *MDMonitorController) runMonitor(ctx context.Context, r controller.Writer, logger *zap.Logger) error {
	return ctrl.MD.Monitor(ctx, func(event string) {
		if !strings.Contains(event, " event detected ") {
			logger.Info("MD monitor message", zap.String("message", event))

			return
		}

		logger.Info("MD monitor event", zap.String("event", event))

		if err := bumpMDRefreshRequest(ctx, r); err != nil {
			logger.Warn("failed to bump MD refresh request", zap.Error(err))
		}
	})
}

func bumpMDRefreshRequest(ctx context.Context, r controller.Writer) error {
	if err := safe.WriterModify(
		ctx,
		r,
		storage.NewMDRefreshRequest(storage.NamespaceName, storage.RefreshID),
		func(rr *storage.MDRefreshRequest) error {
			rr.TypedSpec().Request++

			return nil
		},
	); err != nil {
		return fmt.Errorf("bump MD refresh request: %w", err)
	}

	return nil
}
