// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// VersionController populates the version of currently running Talos.
type VersionController struct{}

// Name implements controller.Controller interface.
func (ctrl *VersionController) Name() string {
	return "runtime.VersionController"
}

// Inputs implements controller.Controller interface.
func (ctrl *VersionController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *VersionController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.VersionType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *VersionController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if err := safe.WriterModify(ctx, r, runtime.NewVersion(), func(status *runtime.Version) error {
		status.TypedSpec().Version = version.Tag

		return nil
	}); err != nil {
		return fmt.Errorf("failed to update version status: %w", err)
	}

	return nil
}
