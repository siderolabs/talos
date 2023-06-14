// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"context"
	"log"
	"os"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader"
	"github.com/siderolabs/talos/internal/pkg/meta"
)

func revertBootloader() {
	if err := revertBootloadInternal(); err != nil {
		log.Printf("failed to revert bootloader: %s", err)
	}
}

//nolint:gocyclo
func revertBootloadInternal() error {
	metaState, err := meta.New(context.Background(), nil)
	if err != nil {
		if os.IsNotExist(err) {
			// no META, no way to revert
			return nil
		}

		return err
	}

	label, ok := metaState.ReadTag(meta.Upgrade)
	if !ok {
		return nil
	}

	if label == "" {
		if _, err = metaState.DeleteTag(context.Background(), meta.Upgrade); err != nil {
			return err
		}

		return metaState.Flush()
	}

	log.Printf("reverting failed upgrade, switching to %q", label)

	if err = func() error {
		config, probeErr := bootloader.Probe("")
		if probeErr != nil {
			if os.IsNotExist(probeErr) {
				// no bootloader found, nothing to do
				return nil
			}

			return probeErr
		}

		return config.Revert()
	}(); err != nil {
		return err
	}

	if _, err = metaState.DeleteTag(context.Background(), meta.Upgrade); err != nil {
		return err
	}

	return metaState.Flush()
}
