// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// cpuStartHold returns why no container may start yet, or an empty string: containers start once
// the CPU partition policy of the boot is applied. A started container is never stopped for it.
func cpuStartHold(ctx context.Context, r controller.Reader) (string, error) {
	status, err := safe.ReaderGetByID[*runtime.CPUPartitionStatus](ctx, r, runtime.CPUPartitionStatusID)
	if err != nil && !state.IsNotFoundError(err) {
		return "", fmt.Errorf("failed to get CPU partition status: %w", err)
	}

	if status == nil || !status.TypedSpec().Ready {
		return "the CPU partition policy of the boot is not applied yet", nil
	}

	return "", nil
}
