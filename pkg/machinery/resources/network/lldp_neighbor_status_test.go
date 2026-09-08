// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func lldpStatus() *network.LLDPNeighborStatus {
	status := network.NewLLDPNeighborStatus(network.NamespaceName, "eth0")
	status.TypedSpec().Neighbors = []network.LLDPNeighborSpec{{
		ChassisID: "02:00:00:00:00:01", SystemName: "switch", SystemDescription: "switch OS",
		PortID: "eth1", PortDescription: "uplink", ManagementAddresses: []string{"192.0.2.1", "2001:db8::1"},
		VLANs: []network.LLDPVLANSpec{{ID: 10, Name: "clients"}, {ID: 20}},
	}}

	return status
}

func TestLLDPNeighborStatusRoundTrip(t *testing.T) {
	t.Parallel()

	status := lldpStatus()
	encoded, err := yaml.Marshal(status.TypedSpec())
	require.NoError(t, err)
	assert.Equal(t, `neighbors:
    - chassisID: '02:00:00:00:00:01'
      systemName: switch
      systemDescription: switch OS
      portID: eth1
      portDescription: uplink
      managementAddresses:
        - 192.0.2.1
        - 2001:db8::1
      vlans:
        - id: 10
          name: clients
        - id: 20
`, string(encoded))

	var decoded network.LLDPNeighborStatusSpec

	require.NoError(t, yaml.Unmarshal(encoded, &decoded))
	assert.Equal(t, *status.TypedSpec(), decoded)

	protoResource, err := protobuf.FromResource(status)
	require.NoError(t, err)
	wire, err := protoResource.Marshal()
	require.NoError(t, err)
	protoResource, err = protobuf.Unmarshal(wire)
	require.NoError(t, err)
	roundTripped, err := protobuf.UnmarshalResource(protoResource)
	require.NoError(t, err)
	assert.True(t, resource.Equal(status, roundTripped))
}

func TestLLDPNeighborStatusDeepCopy(t *testing.T) {
	t.Parallel()

	original := lldpStatus()
	copied := original.DeepCopy().(*network.LLDPNeighborStatus)
	require.True(t, resource.Equal(original, copied))

	copied.TypedSpec().Neighbors[0].SystemName = "other"
	copied.TypedSpec().Neighbors[0].ManagementAddresses[0] = "198.51.100.1"
	copied.TypedSpec().Neighbors[0].VLANs[0].Name = "other"
	assert.Equal(t, lldpStatus().TypedSpec(), original.TypedSpec())
	assert.NotEqual(t, original.TypedSpec(), copied.TypedSpec())
}
