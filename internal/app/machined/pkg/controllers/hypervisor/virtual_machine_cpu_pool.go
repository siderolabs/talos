// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/cpupool"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// resolveCPUPool resolves the CPU pool of a virtual machine once the CPU partition policy of the
// boot is applied.
func resolveCPUPool(ctx context.Context, r controller.Reader, name string, cpu hypervisor.VirtualMachineCPUSpec) (cpupool.Pool, error) {
	status, err := safe.ReaderGetByID[*runtime.CPUPartitionStatus](ctx, r, runtime.CPUPartitionStatusID)
	if err != nil && !state.IsNotFoundError(err) {
		return cpupool.Pool{}, fmt.Errorf("error getting CPU partition status: %w", err)
	}

	var spec *runtime.CPUPartitionStatusSpec
	if status != nil {
		spec = status.TypedSpec()
	}

	return cpupool.Resolve(name, spec, cpu)
}
