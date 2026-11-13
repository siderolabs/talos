// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/cpupool"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// resolveCPUPool resolves the libvirt resource partition of a virtual machine once the CPU partition
// policy of the boot is applied.
func resolveCPUPool(ctx context.Context, r controller.Reader, name string, cpu hypervisor.VirtualMachineCPUSpec) (string, error) {
	status, err := runtime.ReadyCPUPartitionStatus(ctx, r)
	if err != nil {
		return "", err
	}

	if status == nil {
		return "", fmt.Errorf("virtual machine %q: %w", name, cpupool.ErrPending)
	}

	return cpupool.Select(name, status.VirtualMachinePools, cpu)
}
