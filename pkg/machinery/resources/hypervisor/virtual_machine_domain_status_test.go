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

	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestVirtualMachineDomainStatusRoundTrip(t *testing.T) {
	t.Parallel()

	status := hypervisor.NewVirtualMachineDomainStatus(hypervisor.NamespaceName, "external")
	status.TypedSpec().UUID = "c737f778-82a1-48dd-990b-67901031bcc5"
	status.TypedSpec().PowerState = hypervisor.VirtualMachinePowerStateRunning
	status.TypedSpec().State = 1
	status.TypedSpec().MaxMemoryKiB = 1048576
	status.TypedSpec().MemoryKiB = 524288
	status.TypedSpec().VCPUs = 2
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

	decoded, err := protobuf.Unmarshal(wire)
	require.NoError(t, err)

	resource, err := protobuf.UnmarshalResource(decoded)
	require.NoError(t, err)
	require.Equal(t, status.TypedSpec(), resource.(*hypervisor.VirtualMachineDomainStatus).TypedSpec())

	text, err := yaml.Marshal(status.TypedSpec())
	require.NoError(t, err)
	require.Contains(t, string(text), "powerState: running")
	require.Contains(t, string(text), "10.0.0.5/24")
	require.Contains(t, string(text), "2001:db8::5/64")
}
