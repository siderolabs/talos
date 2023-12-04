// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"bytes"
	"context"
	"os"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// WaitForDevicesReady waits for devices to be ready.
//
// It is a helper function for controllers.
func WaitForDevicesReady(ctx context.Context, r controller.Runtime, nextInputs []controller.Input) error {
	// set inputs temporarily to a service only
	if err := r.UpdateInputs([]controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.DevicesStatusType,
			ID:        optional.Some(runtime.DevicesID),
			Kind:      controller.InputWeak,
		},
	}); err != nil {
		return err
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-r.EventCh():
		}

		status, err := safe.ReaderGetByID[*runtime.DevicesStatus](ctx, r, runtime.DevicesID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if status.TypedSpec().Ready {
			// condition met
			break
		}
	}

	// restore inputs
	if err := r.UpdateInputs(nextInputs); err != nil {
		return err
	}

	// queue an update to reprocess with new inputs
	r.QueueReconcile()

	return nil
}

// updateFile is like `os.WriteFile`, but it will only update the file if the
// contents have changed.
func updateFile(filename string, contents []byte, mode os.FileMode) error {
	oldContents, err := os.ReadFile(filename)
	if err == nil && bytes.Equal(oldContents, contents) {
		return nil
	}

	return os.WriteFile(filename, contents, mode)
}
