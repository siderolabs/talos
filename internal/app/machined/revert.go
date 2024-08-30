// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/options"
	"github.com/siderolabs/talos/internal/pkg/meta"
	metaconsts "github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

var revertState state.State

func revertSetState(s state.State) {
	revertState = s
}

func revertBootloader(ctx context.Context) {
	if revertState == nil {
		log.Printf("no state to revert bootloader")

		return
	}

	if err := revertBootloadInternal(ctx, revertState); err != nil {
		log.Printf("failed to revert bootloader: %s", err)
	}
}

//nolint:gocyclo
func revertBootloadInternal(ctx context.Context, resourceState state.State) error {
	systemDisk, err := block.GetSystemDisk(ctx, resourceState)
	if err != nil {
		return fmt.Errorf("system disk lookup failed: %w", err)
	}

	if systemDisk == nil {
		log.Printf("no system disk found, nothing to revert")

		return nil
	}

	metaState, err := meta.New(ctx, resourceState)
	if err != nil {
		if os.IsNotExist(err) {
			// no META, no way to revert
			return nil
		}

		return err
	}

	label, ok := metaState.ReadTag(metaconsts.Upgrade)
	if !ok {
		return nil
	}

	if label == "" {
		if _, err = metaState.DeleteTag(ctx, metaconsts.Upgrade); err != nil {
			return err
		}

		return metaState.Flush()
	}

	log.Printf("reverting failed upgrade, switching to %q", label)

	if err := func() error {
		config, err := bootloader.Probe(systemDisk.DevPath, options.ProbeOptions{})
		if err != nil {
			return err
		}

		return config.Revert(systemDisk.DevPath)
	}(); err != nil {
		return err
	}

	if _, err = metaState.DeleteTag(ctx, metaconsts.Upgrade); err != nil {
		return err
	}

	return metaState.Flush()
}
