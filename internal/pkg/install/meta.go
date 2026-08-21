// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	blockres "github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// metaWaitTimeout bounds the wait for the META partition created by the installer to become a ready
// volume.
const metaWaitTimeout = time.Minute

// ReloadMeta reloads the in-memory META from the META partition created by the installer, merging
// the in-memory values on top of the on-disk ones.
func ReloadMeta(ctx context.Context, st state.State, meta runtime.Meta) error {
	ctx, cancel := context.WithTimeout(ctx, metaWaitTimeout)
	defer cancel()

	if err := waitForMeta(ctx, st); err != nil {
		return err
	}

	if err := meta.Reload(ctx); err != nil {
		return fmt.Errorf("failed to reload META: %w", err)
	}

	return nil
}

// SyncMeta writes the in-memory META to the META partition created by the installer.
//
// The installer creates the META partition from scratch, so in-memory META values are lost unless
// they are explicitly flushed: e.g. the values seeded from `talos.environment=INSTALLER_META_BASE64=...`
// while the machine was running in maintenance mode, or any tag set at runtime via the META API.
//
// Call ReloadMeta first to merge in the values the installer itself has written, otherwise they are
// overwritten by the in-memory contents.
func SyncMeta(ctx context.Context, st state.State, meta runtime.Meta) error {
	ctx, cancel := context.WithTimeout(ctx, metaWaitTimeout)
	defer cancel()

	if err := waitForMeta(ctx, st); err != nil {
		return err
	}

	if err := meta.Flush(); err != nil {
		return fmt.Errorf("failed to flush META: %w", err)
	}

	return nil
}

func waitForMeta(ctx context.Context, st state.State) error {
	if _, err := blockres.WaitForVolumePhase(ctx, st, constants.MetaPartitionLabel, blockres.VolumePhaseReady); err != nil {
		return fmt.Errorf("failed to wait for the META volume: %w", err)
	}

	return nil
}
