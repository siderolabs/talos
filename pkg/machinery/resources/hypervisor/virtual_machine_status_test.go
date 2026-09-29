// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"
	"google.golang.org/protobuf/proto"

	enumsapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/enums"
	hypervisorapi "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineStatusRoundTrip(t *testing.T) {
	t.Parallel()

	status := hypervisor.NewVirtualMachineStatus(hypervisor.NamespaceName, "vm1")
	status.TypedSpec().PowerState = hypervisor.VirtualMachinePowerStateRunning
	status.TypedSpec().Stage = hypervisor.VirtualMachineStageReady

	encoded, err := protobuf.FromResource(status)
	require.NoError(t, err)

	wire, err := encoded.Marshal()
	require.NoError(t, err)

	var spec hypervisorapi.VirtualMachineStatusSpec
	require.NoError(t, proto.Unmarshal(wire.Spec.ProtoSpec, &spec))
	require.Equal(t, enumsapi.HypervisorVirtualMachinePowerState_VIRTUAL_MACHINE_POWER_STATE_RUNNING, spec.GetPowerState())
	require.Equal(t, enumsapi.HypervisorVirtualMachineStage_VIRTUAL_MACHINE_STAGE_READY, spec.GetStage())

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)

	resource, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.Equal(t, status.TypedSpec(), resource.(*hypervisor.VirtualMachineStatus).TypedSpec())

	text, err := yaml.Marshal(status.TypedSpec())
	require.NoError(t, err)
	require.Contains(t, string(text), "powerState: running")
	require.Contains(t, string(text), "stage: ready")
}
