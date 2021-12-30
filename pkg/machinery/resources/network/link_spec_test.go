// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/nethelpers"
	"github.com/talos-systems/talos/pkg/machinery/resources/network"
)

func TestLinkSpecMarshalYAML(t *testing.T) {
	spec := network.LinkSpecSpec{
		Name:       "eth0",
		Logical:    true,
		Up:         true,
		MTU:        1437,
		Kind:       "eth",
		Type:       nethelpers.LinkEther,
		ParentName: "eth1",
		MasterName: "bond0",
		VLAN: network.VLANSpec{
			VID:      25,
			Protocol: nethelpers.VLANProtocol8021AD,
		},
		BondMaster: network.BondMasterSpec{
			Mode:            nethelpers.BondMode8023AD,
			HashPolicy:      nethelpers.BondXmitPolicyEncap34,
			LACPRate:        nethelpers.LACPRateFast,
			ARPValidate:     nethelpers.ARPValidateAll,
			ARPAllTargets:   nethelpers.ARPAllTargetsAny,
			PrimaryIndex:    3,
			PrimaryReselect: nethelpers.PrimaryReselectBetter,
			FailOverMac:     nethelpers.FailOverMACFollow,
			ADSelect:        nethelpers.ADSelectCount,
			MIIMon:          33,
			UpDelay:         100,
			DownDelay:       200,
			ARPInterval:     10,
			ResendIGMP:      30,
			MinLinks:        1,
			LPInterval:      3,
			PacketsPerSlave: 4,
			NumPeerNotif:    4,
			TLBDynamicLB:    5,
			AllSlavesActive: 1,
			UseCarrier:      true,
			ADActorSysPrio:  6,
			ADUserPortKey:   7,
			PeerNotifyDelay: 40,
		},
		Wireguard: network.WireguardSpec{
			PrivateKey:   "foo=",
			PublicKey:    "bar=",
			ListenPort:   51820,
			FirewallMark: 11233,
			Peers: []network.WireguardPeer{
				{
					PublicKey:                   "peer=",
					PresharedKey:                "key=",
					Endpoint:                    "127.0.0.1:3333",
					PersistentKeepaliveInterval: 30 * time.Second,
					AllowedIPs: []netaddr.IPPrefix{
						netaddr.MustParseIPPrefix("192.83.93.94/31"),
					},
				},
			},
		},
		ConfigLayer: network.ConfigPlatform,
	}

	marshaled, err := yaml.Marshal(spec)
	require.NoError(t, err)

	assert.Equal(t,
		`name: eth0
logical: true
up: true
mtu: 1437
kind: eth
type: ether
parentName: eth1
masterName: bond0
vlan:
    vlanID: 25
    vlanProtocol: 802.1ad
bondMaster:
    mode: 802.3ad
    xmitHashPolicy: encap3+4
    lacpRate: fast
    arpValidate: all
    arpAllTargets: any
    primary: 3
    primaryReselect: better
    failOverMac: 2
    adSelect: count
    miimon: 33
    updelay: 100
    downdelay: 200
    arpInterval: 10
    resendIgmp: 30
    minLinks: 1
    lpInterval: 3
    packetsPerSlave: 4
    numPeerNotif: 4
    tlbLogicalLb: 5
    allSlavesActive: 1
    useCarrier: true
    adActorSysPrio: 6
    adUserPortKey: 7
    peerNotifyDelay: 40
wireguard:
    privateKey: foo=
    publicKey: bar=
    listenPort: 51820
    firewallMark: 11233
    peers:
        - publicKey: peer=
          presharedKey: key=
          endpoint: 127.0.0.1:3333
          persistentKeepaliveInterval: 30s
          allowedIPs:
            - 192.83.93.94/31
layer: platform
`,
		string(marshaled))

	var spec2 network.LinkSpecSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}
