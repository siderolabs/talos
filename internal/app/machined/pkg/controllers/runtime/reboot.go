// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// RebootController watches RebootRequest resources and performs the actual reboot.
//
// Any controller that needs to trigger a reboot should create or update a RebootRequest resource
// instead of calling the reboot function directly. This allows multiple controllers to share
// the same reboot mechanism.
type RebootController struct {
	V1Alpha1Mode v1alpha1runtime.Mode

	PreRebootFunc func(ctx context.Context) error

	Reboot func(ctx context.Context) error
}

// NewRebootController creates a RebootController wired to the runtime.
func NewRebootController(rt v1alpha1runtime.Runtime, reboot func(ctx context.Context) error) *RebootController {
	return &RebootController{
		V1Alpha1Mode: rt.State().Platform().Mode(),
		Reboot:       reboot,
		PreRebootFunc: func(ctx context.Context) error {
			for {
				if err := rt.State().Machine().Meta().Reload(ctx); err != nil {
					if errors.Is(err, fs.ErrNotExist) {
						select {
						case <-time.After(500 * time.Millisecond):
							continue
						case <-ctx.Done():
							return fmt.Errorf("timed out waiting for META: %w", ctx.Err())
						}
					}

					return fmt.Errorf("failed to reload META: %w", err)
				}

				break
			}

			if err := rt.State().Machine().Meta().Flush(); err != nil {
				return fmt.Errorf("failed to flush META: %w", err)
			}

			return nil
		},
	}
}

// Name implements controller.Controller interface.
func (ctrl *RebootController) Name() string {
	return "runtime.RebootController"
}

// Inputs implements controller.Controller interface.
func (ctrl *RebootController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.RebootRequestType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *RebootController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *RebootController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// no reboot in container mode.
	if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		req, err := safe.ReaderGetByID[*runtime.RebootRequest](ctx, r, runtime.RebootRequestID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		// RebootRequest exists: trigger reboot.
		_ = req // silence unused-variable lint if we ever add spec fields.

		logger.Info("reboot requested via RebootRequest resource")

		if err := ctrl.PreRebootFunc(ctx); err != nil {
			logger.Error("failed to flush META before reboot", zap.Error(err))
		}

		go func() {
			if err := ctrl.Reboot(ctx); err != nil {
				logger.Error("failed to reboot", zap.Error(err))
			}
		}()

		// Return so the controller stops processing events — the reboot will happen.
		return nil
	}
}
