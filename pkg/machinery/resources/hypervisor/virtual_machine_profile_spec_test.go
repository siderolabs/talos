// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineProfileResourceRoundTrip(t *testing.T) {
	t.Parallel()

	profile := hypervisor.NewVirtualMachineProfileSpec(hypervisor.NamespaceName, "rack-a")
	profile.TypedSpec().Machine = "pc-q35-9.2"
	profile.TypedSpec().CPU.Mode = hypervisorhelpers.VirtualMachineProfileCPUModeCustom
	profile.TypedSpec().CPU.Model = "x86-64-v3"
	profile.TypedSpec().CPU.Fallback = "forbid"
	profile.TypedSpec().CPU.Features = []hypervisor.VirtualMachineProfileCPUFeatureSpec{{Name: "pku", Policy: hypervisorhelpers.VirtualMachineProfileCPUFeaturePolicyDisable}}
	encoded, err := protobuf.FromResource(profile)
	require.NoError(t, err)
	wire, err := encoded.Marshal()
	require.NoError(t, err)
	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)
	value, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.Equal(t, profile.TypedSpec(), value.(*hypervisor.VirtualMachineProfileSpec).TypedSpec())
	text, err := yaml.Marshal(profile.TypedSpec())
	require.NoError(t, err)
	require.Contains(t, string(text), "mode: custom")
	require.Contains(t, string(text), "policy: disable")
}
