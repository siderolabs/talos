// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/siderolabs/protoenc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	networkpb "github.com/siderolabs/talos/pkg/machinery/api/resource/definitions/network"
	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestRegisterResource(t *testing.T) {
	t.Parallel()

	ctx := t.Context()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []meta.ResourceWithRD{
		&network.AddressStatus{},
		&network.AddressSpec{},
		&network.BGPInstanceConfig{},
		&network.HardwareAddr{},
		&network.DNSUpstream{},
		&network.EthernetSpec{},
		&network.EthernetStatus{},
		&network.HostDNSConfig{},
		&network.HostnameStatus{},
		&network.HostnameSpec{},
		&network.LinkAliasSpec{},
		&network.LinkRefresh{},
		&network.LinkStatus{},
		&network.LinkSpec{},
		&network.NfTablesChain{},
		&network.NodeAddress{},
		&network.NodeAddressFilter{},
		&network.NodeAddressSortAlgorithm{},
		&network.OperatorSpec{},
		&network.PlatformConfig{},
		&network.ProbeSpec{},
		&network.ResolverStatus{},
		&network.ResolverSpec{},
		&network.RouteStatus{},
		&network.RouteSpec{},
		&network.StaticHost{},
		&network.Status{},
		&network.TimeServerStatus{},
		&network.TimeServerSpec{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}

func TestProtobufInterop(t *testing.T) {
	t.Parallel()

	// TODO: this should be auto-generated, but for now we just want to fix the bug and add regression
	for _, test := range []struct {
		res interface {
			resource.Resource
			ResourceDefinition() meta.ResourceDefinitionSpec
		}
		spec proto.Message
	}{
		{
			res:  &network.AddressStatus{},
			spec: &networkpb.AddressStatusSpec{},
		},
		{
			res:  &network.EthernetStatus{},
			spec: &networkpb.EthernetStatusSpec{},
		},
		{
			res:  &network.LinkSpec{},
			spec: &networkpb.LinkSpecSpec{},
		},
		{
			res:  &network.LinkStatus{},
			spec: &networkpb.LinkStatusSpec{},
		},
		{
			res:  &network.NfTablesChain{},
			spec: &networkpb.NfTablesChainSpec{},
		},
		{
			res:  &network.OperatorSpec{},
			spec: &networkpb.OperatorSpecSpec{},
		},
	} {
		t.Run(test.res.ResourceDefinition().Type, func(t *testing.T) {
			t.Parallel()

			require.NoError(t, proto.ResourceSpecToProto(test.res, test.spec, protoenc.WithMarshalZeroFields()))
		})
	}
}

// TestOperatorSpecDHCP4SkipRoutesProtobuf is a regression test for the DHCP4
// SkipRoutes field being dropped when an OperatorSpec crosses the resource API:
// the field is present on the Go resource struct but must also exist in the
// generated protobuf bindings, or it is silently lost on the wire.
func TestOperatorSpecDHCP4SkipRoutesProtobuf(t *testing.T) {
	t.Parallel()

	res := network.NewOperatorSpec(network.NamespaceName, "test")
	res.TypedSpec().Operator = network.OperatorDHCP4
	res.TypedSpec().DHCP4.SkipRoutes = true

	var spec networkpb.OperatorSpecSpec

	require.NoError(t, proto.ResourceSpecToProto(res, &spec))

	assert.True(t, spec.GetDhcp4().GetSkipRoutes(), "SkipRoutes must survive the resource->protobuf roundtrip")
}
