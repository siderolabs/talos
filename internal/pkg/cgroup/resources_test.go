// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup //nolint:testpackage

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TestWorkloadRootsEnablePidsUnlimited checks the workload roots get a pids.max file at boot, left at "max".
//
// Without the pids controller enabled in the parent, pids.max does not exist and WorkloadPidsController
// cannot reconcile it.
func TestWorkloadRootsEnablePidsUnlimited(t *testing.T) {
	t.Parallel()

	for _, name := range []string{constants.CgroupTalosContainersRoot, constants.CgroupVirtualMachines, constants.CgroupExtensions} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			resources := getCgroupV2Resources(name)

			assert.Contains(t, resources.EnabledControllers(), "pids")

			// pids.max is left to WorkloadPidsController
			require.NotNil(t, resources.Pids)
			assert.Empty(t, resources.Pids.Values())
		})
	}
}
