// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// SwapStatusController provides a view of active swap devices.
type SwapStatusController struct {
	V1Alpha1Mode  machineruntime.Mode
	ProcSwapsPath string
}

// Name implements controller.Controller interface.
func (ctrl *SwapStatusController) Name() string {
	return "block.SwapStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SwapStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			// not really a dependency, but we refresh swap status on mount status change
			Namespace: block.NamespaceName,
			Type:      block.MountStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SwapStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.SwapStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *SwapStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// in container mode, no swap applies
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	if ctrl.ProcSwapsPath == "" {
		ctrl.ProcSwapsPath = "/proc/swaps"
	}

	// there is no way to watch for swap devices, so we are going to poll every minute
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		r.StartTrackingOutputs()

		if err := ctrl.parseSwaps(ctx, r); err != nil {
			return fmt.Errorf("failed to parse swaps: %w", err)
		}

		if err := safe.CleanupOutputs[*block.SwapStatus](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}

func (ctrl *SwapStatusController) parseSwaps(ctx context.Context, r controller.ReaderWriter) error {
	swapsContent, err := os.ReadFile(ctrl.ProcSwapsPath)
	if err != nil {
		return fmt.Errorf("failed to read %q: %w", ctrl.ProcSwapsPath, err)
	}

	scanner := bufio.NewScanner(bytes.NewReader(swapsContent))

	// skip the first line, it contains headers
	if !scanner.Scan() {
		return fmt.Errorf("failed to read header line from %q: %w", ctrl.ProcSwapsPath, scanner.Err())
	}

	for scanner.Scan() {
		line := scanner.Text()

		fields := strings.Fields(line)
		if len(fields) < 5 {
			return fmt.Errorf("invalid swap line in %q: %q", ctrl.ProcSwapsPath, line)
		}

		swapDevice := fields[0]

		if err = safe.WriterModify(ctx, r, block.NewSwapStatus(block.NamespaceName, swapDevice),
			func(swapStatus *block.SwapStatus) error {
				swapStatus.TypedSpec().Device = swapDevice
				swapStatus.TypedSpec().Type = fields[1]

				size, err := strconv.ParseUint(fields[2], 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse size from %q: %w", fields[2], err)
				}

				swapStatus.TypedSpec().SizeBytes = size * 1024 // convert from KiB to bytes

				used, err := strconv.ParseUint(fields[3], 10, 64)
				if err != nil {
					return fmt.Errorf("failed to parse used from %q: %w", fields[3], err)
				}

				swapStatus.TypedSpec().UsedBytes = used * 1024 // convert from KiB to bytes

				swapStatus.TypedSpec().SizeHuman = humanize.IBytes(swapStatus.TypedSpec().SizeBytes)
				swapStatus.TypedSpec().UsedHuman = humanize.IBytes(swapStatus.TypedSpec().UsedBytes)

				priority, err := strconv.ParseInt(fields[4], 10, 32)
				if err != nil {
					return fmt.Errorf("failed to parse priority from %q: %w", fields[4], err)
				}

				swapStatus.TypedSpec().Priority = int32(priority)

				return nil
			},
		); err != nil {
			return fmt.Errorf("failed to modify swap status for %q: %w", swapDevice, err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read swaps from %q: %w", ctrl.ProcSwapsPath, err)
	}

	return nil
}
