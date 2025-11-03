// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	"net"
	"net/netip"
	"testing"
	"time"

	"github.com/mdlayher/ethtool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

func TestLinkStatusMarshalYAML(t *testing.T) {
	hwAddr, err := net.ParseMAC("01:23:45:67:89:ab")
	require.NoError(t, err)

	bcAddr, err := net.ParseMAC("ff:ff:ff:ff:ff:ff")
	require.NoError(t, err)

	spec := network.LinkStatusSpec{
		Index:            3,
		Type:             nethelpers.LinkEther,
		LinkIndex:        44,
		Flags:            nethelpers.LinkFlags(nethelpers.LinkUp | nethelpers.LinkRunning),
		HardwareAddr:     nethelpers.HardwareAddr(hwAddr),
		PermanentAddr:    nethelpers.HardwareAddr(hwAddr),
		BroadcastAddr:    nethelpers.HardwareAddr(bcAddr),
		MTU:              1500,
		QueueDisc:        "fifo",
		MasterIndex:      4,
		OperationalState: nethelpers.OperStateLowerLayerDown,
		Kind:             "bridge",
		SlaveKind:        "ether",
		BusPath:          "00:11:22",
		PCIID:            "0000:00:00.0",
		Driver:           "bonding",
		DriverVersion:    "1.0.0",
		FirmwareVersion:  "3.1.5",
		ProductID:        "0x3ebf",
		VendorID:         "0x1d6b",
		Product:          "10Gbase-T",
		Vendor:           "Intel Corporation",
		LinkState:        true,
		SpeedMegabits:    1024,
		Port:             nethelpers.Port(ethtool.TwistedPair),
		Duplex:           nethelpers.Duplex(ethtool.Full),
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
			PublicKey:    "bar=",
			ListenPort:   51820,
			FirewallMark: 11233,
			Peers: []network.WireguardPeer{
				{
					PublicKey:                   "peer=",
					PresharedKey:                "key=",
					Endpoint:                    "127.0.0.1:3333",
					PersistentKeepaliveInterval: 30 * time.Second,
					AllowedIPs: []netip.Prefix{
						netip.MustParsePrefix("192.83.93.94/31"),
					},
				},
			},
		},
	}

	marshaled, err := yaml.Marshal(spec)
	require.NoError(t, err)

	assert.Equal(t,
		`index: 3
type: ether
linkIndex: 44
flags: UP,RUNNING
hardwareAddr: 01:23:45:67:89:ab
permanentAddr: 01:23:45:67:89:ab
broadcastAddr: ff:ff:ff:ff:ff:ff
mtu: 1500
queueDisc: fifo
masterIndex: 4
operationalState: lowerLayerDown
kind: bridge
slaveKind: ether
busPath: "00:11:22"
pciID: "0000:00:00.0"
driver: bonding
driverVersion: 1.0.0
firmwareVersion: 3.1.5
productID: "0x3ebf"
vendorID: "0x1d6b"
product: 10Gbase-T
vendor: Intel Corporation
linkState: true
speedMbit: 1024
port: TwistedPair
duplex: Full
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
`,
		string(marshaled))

	var spec2 network.LinkStatusSpec

	require.NoError(t, yaml.Unmarshal(marshaled, &spec2))

	assert.Equal(t, spec, spec2)
}
