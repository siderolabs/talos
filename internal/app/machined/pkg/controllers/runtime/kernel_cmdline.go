// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

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

// KernelCmdlineController presents /proc/cmdline as a resource.
type KernelCmdlineController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *KernelCmdlineController) Name() string {
	return "runtime.KernelCmdlineController"
}

// Inputs implements controller.Controller interface.
func (ctrl *KernelCmdlineController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *KernelCmdlineController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.KernelCmdlineType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *KernelCmdlineController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode.InContainer() {
		// no cmdline in containers
		return nil
	}

	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	contents, err := os.ReadFile("/proc/cmdline")
	if err != nil {
		return fmt.Errorf("error reading /proc/cmdline: %w", err)
	}

	if err := safe.WriterModify(ctx, r,
		runtime.NewKernelCmdline(),
		func(res *runtime.KernelCmdline) error {
			res.TypedSpec().Cmdline = strings.TrimSpace(string(contents))

			return nil
		},
	); err != nil {
		return fmt.Errorf("error updating KernelCmdline resource: %w", err)
	}

	return nil
}
