// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl
package runtime

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// BootIDController presents /proc/sys/kernel/random/boot_id as a resource.
type BootIDController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *BootIDController) Name() string {
	return "runtime.BootIDController"
}

// Inputs implements controller.Controller interface.
func (ctrl *BootIDController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *BootIDController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.BootIDType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *BootIDController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		// no boot_id in containers
		return nil
	}

	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	contents, err := os.ReadFile("/proc/sys/kernel/random/boot_id")
	if err != nil {
		return fmt.Errorf("error reading boot_id: %w", err)
	}

	if err := safe.WriterModify(
		ctx, r,
		runtime.NewBootID(),
		func(res *runtime.BootID) error {
			res.TypedSpec().BootID = strings.TrimSpace(string(contents))

			return nil
		},
	); err != nil {
		return fmt.Errorf("error updating BootID resource: %w", err)
	}

	return nil
}
