// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm_test

import (
	"net"
	"testing"

	"github.com/gopacket/gopacket"
	"github.com/gopacket/gopacket/layers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/provision/providers/vm"
)

func TestLLDPTestFrame(t *testing.T) {
	packet := gopacket.NewPacket(vm.LLDPTestFrame(), layers.LayerTypeEthernet, gopacket.Default)
	require.Nil(t, packet.ErrorLayer())

	ethernet, ok := packet.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
	require.True(t, ok)
	assert.Equal(t, "01:80:c2:00:00:0e", ethernet.DstMAC.String())

	lldp, ok := packet.Layer(layers.LayerTypeLinkLayerDiscovery).(*layers.LinkLayerDiscovery)
	require.True(t, ok)
	assert.Equal(t, "02:00:00:00:00:01", net.HardwareAddr(lldp.ChassisID.ID).String())
	assert.Equal(t, "swp-test1", string(lldp.PortID.ID))
	assert.EqualValues(t, 30, lldp.TTL)

	info, ok := packet.Layer(layers.LayerTypeLinkLayerDiscoveryInfo).(*layers.LinkLayerDiscoveryInfo)
	require.True(t, ok)
	assert.Equal(t, "switch.example.com", info.SysName)
	assert.Equal(t, "test switch", info.SysDescription)
	assert.Equal(t, "Ethernet test port", info.PortDescription)

	var managementAddresses []string

	for _, value := range lldp.Values {
		if value.Type == layers.LLDPTLVMgmtAddress {
			managementAddresses = append(managementAddresses, net.IP(value.Value[2:1+int(value.Value[0])]).String())
		}
	}

	assert.Equal(t, []string{"192.0.2.1", "2001:db8::1"}, managementAddresses)

	vlanInfo, err := info.Decode8021()
	require.NoError(t, err)
	assert.Equal(t, []layers.VLANName{{ID: 100, Name: "prod"}, {ID: 200, Name: "storage"}}, vlanInfo.VLANNames)
}
