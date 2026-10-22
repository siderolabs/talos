// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"net/netip"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
	"google.golang.org/protobuf/proto"

	enumsapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/enums"
	hypervisorapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineStatusRoundTrip(t *testing.T) {
	t.Parallel()

	status := hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, "vm1")
	status.TypedSpec().PowerState = hypervisorhelpers.VirtualMachinePowerStateRunning
	status.TypedSpec().Stage = hypervisorhelpers.VirtualMachineStageReady
	status.TypedSpec().ObservedGeneration = "7"
	status.TypedSpec().Interfaces = []hypervisor.VirtualMachineGuestInterfaceSpec{
		{
			Name: "eth0",
			IPAddresses: []netip.Prefix{
				netip.MustParsePrefix("10.0.0.5/24"),
				netip.MustParsePrefix("2001:db8::5/64"),
			},
		},
	}

	encoded, err := protobuf.FromResource(status)
	require.NoError(t, err)

	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec hypervisorapi.VirtualMachineStatusSpec
	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	require.Equal(t, enumsapi.HypervisorhelpersVirtualMachinePowerState_VIRTUAL_MACHINE_POWER_STATE_RUNNING, spec.GetPowerState())
	require.Equal(t, enumsapi.HypervisorhelpersVirtualMachineStage_VIRTUAL_MACHINE_STAGE_READY, spec.GetStage())
	require.Equal(t, "7", spec.GetObservedGeneration())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)

	resource, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.Equal(t, status.TypedSpec(), resource.(*hypervisor.VirtualMachineStatus).TypedSpec())

	text, err := yaml.Marshal(status.TypedSpec())
	require.NoError(t, err)
	require.Contains(t, string(text), "powerState: running")
	require.Contains(t, string(text), "stage: ready")
	require.Contains(t, string(text), `observedGeneration: "7"`)
	require.Contains(t, string(text), "10.0.0.5/24")
	require.Contains(t, string(text), "2001:db8::5/64")
}
