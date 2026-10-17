// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestKernelModuleLoadOrder(t *testing.T) {
	t.Parallel()

	spec := func(name string, order int) *runtime.KernelModuleSpec {
		res := runtime.NewKernelModuleSpec(runtime.NamespaceName, name)
		res.TypedSpec().Name = name
		res.TypedSpec().Order = order

		return res
	}

	// Listed by ID, the way the resource state returns them.
	modules := []*runtime.KernelModuleSpec{
		spec("earth-pt3", 3),
		spec("qm1d1c0042", 2),
		spec("raid1", 0),
		spec("tc90522", 1),
		spec("vrf", 0),
	}

	names := make([]string, 0, len(modules))

	for _, module := range kernelModuleLoadOrder(slices.Values(modules)) {
		names = append(names, module.TypedSpec().Name)
	}

	// Modules requested by Talos first (by name), then the machine config order:
	// earth-pt3 requests tc90522 and qm1d1c0042 while probing, so it goes last.
	assert.Equal(t, []string{"raid1", "vrf", "tc90522", "qm1d1c0042", "earth-pt3"}, names)
}
