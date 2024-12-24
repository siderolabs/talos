// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/siderolabs/go-cmd/pkg/cmd"
	"go.uber.org/zap"
)

// FDSpyController activates LVM volumes when they are discovered by the block.DiscoveryController.
type FDSpyController struct {
}

// Name implements controller.Controller interface.
func (ctrl *FDSpyController) Name() string {
	return "runtime.FDSpyController"
}

// Inputs implements controller.Controller interface.
func (ctrl *FDSpyController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *FDSpyController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *FDSpyController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		if _, err := cmd.RunContext(ctx,
			"/usr/bin/fdspy",
		); err != nil {
			return fmt.Errorf("failed to run fdspy: %w", err)
		}
	}
}
