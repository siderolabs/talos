// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/dustin/go-humanize"
	"go.uber.org/zap"

	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ZswapStatusController provides a view of active swap devices.
type ZswapStatusController struct {
	V1Alpha1Mode machineruntime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *ZswapStatusController) Name() string {
	return "block.ZswapStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ZswapStatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			// not really a dependency, but we refresh zswap status kernel param change
			Namespace: runtime.NamespaceName,
			Type:      runtime.KernelParamStatusType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ZswapStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.ZswapStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ZswapStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// in container mode, no zswap applies
	if ctrl.V1Alpha1Mode == machineruntime.ModeContainer {
		return nil
	}

	// there is no way to watch for zswap status, so we are going to poll every minute
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

		// try to read a single status file to see if zswap is enabled
		if _, err := os.ReadFile("/sys/kernel/debug/zswap/pool_total_size"); err == nil {
			if err = safe.WriterModify(ctx, r, block.NewZswapStatus(block.NamespaceName, block.ZswapStatusID),
				func(zs *block.ZswapStatus) error {
					for _, p := range []struct {
						name string
						out  *uint64
					}{
						{"pool_total_size", &zs.TypedSpec().TotalSizeBytes},
						{"stored_pages", &zs.TypedSpec().StoredPages},
						{"pool_limit_hit", &zs.TypedSpec().PoolLimitHit},
						{"reject_reclaim_fail", &zs.TypedSpec().RejectReclaimFail},
						{"reject_alloc_fail", &zs.TypedSpec().RejectAllocFail},
						{"reject_kmemcache_fail", &zs.TypedSpec().RejectKmemcacheFail},
						{"reject_compress_fail", &zs.TypedSpec().RejectCompressFail},
						{"reject_compress_poor", &zs.TypedSpec().RejectCompressPoor},
						{"written_back_pages", &zs.TypedSpec().WrittenBackPages},
					} {
						if err := ctrl.readZswapParam(p.name, p.out); err != nil {
							return err
						}
					}

					zs.TypedSpec().TotalSizeHuman = humanize.IBytes(zs.TypedSpec().TotalSizeBytes)

					return nil
				},
			); err != nil {
				return fmt.Errorf("failed to create zswap status: %w", err)
			}
		}

		if err := safe.CleanupOutputs[*block.ZswapStatus](ctx, r); err != nil {
			return fmt.Errorf("failed to cleanup outputs: %w", err)
		}
	}
}

func (ctrl *ZswapStatusController) readZswapParam(name string, out *uint64) error {
	content, err := os.ReadFile(filepath.Join("/sys/kernel/debug/zswap", name))
	if err != nil {
		return fmt.Errorf("failed to read zswap parameter %q: %w", name, err)
	}

	*out, err = strconv.ParseUint(string(bytes.TrimSpace(content)), 10, 64)
	if err != nil {
		return fmt.Errorf("failed to parse zswap parameter %q: %w", name, err)
	}

	return nil
}
