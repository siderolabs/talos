// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineLifecycleRegister(t *testing.T) {
	t.Parallel()

	resources := state.WrapCore(inmem.NewState())

	require.NoError(t, registry.NewResourceRegistry(resources).Register(t.Context(), &hypervisor.VirtualMachineLifecycle{}))
}

func TestVirtualMachineLifecycleRoundTrip(t *testing.T) {
	t.Parallel()

	res := hypervisor.NewVirtualMachineLifecycle(hypervisor.NamespaceName, hypervisor.VirtualMachineLifecycleID)
	res.TypedSpec().ShutdownDeadline = time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)

	encoded, err := protobuf.FromResource(res)
	require.NoError(t, err)

	wire, err := encoded.Marshal()
	require.NoError(t, err)

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)

	roundTrip, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.IsType(t, res, roundTrip)
	assert.Equal(t, res.TypedSpec(), roundTrip.(*hypervisor.VirtualMachineLifecycle).TypedSpec())
}
