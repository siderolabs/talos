// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
)

// cleanupOutputs tears down every output of type T which wanted no longer names, or which is already
// leaving, and destroys the ones nothing holds any more. what names the resource in the error
// messages. Unlike safe.CleanupOutputs, which destroys without tearing down first, this leaves a
// controller holding the output a moment to notice and give its hold back.
func cleanupOutputs[T generic.ResourceWithRD](
	ctx context.Context, r controller.ReaderWriter, what string, wanted map[resource.ID]struct{},
) error {
	outputs, err := safe.ReaderListAll[T](ctx, r)
	if err != nil {
		return fmt.Errorf("failed to list %s resources: %w", what, err)
	}

	for output := range outputs.All() {
		md := output.Metadata()

		if _, keep := wanted[md.ID()]; keep && md.Phase() == resource.PhaseRunning {
			continue
		}

		okToDestroy, err := r.Teardown(ctx, md)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to tear down %s %q: %w", what, md.ID(), err)
		}

		if !okToDestroy {
			continue
		}

		if err := r.Destroy(ctx, md); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to destroy %s %q: %w", what, md.ID(), err)
		}
	}

	return nil
}
