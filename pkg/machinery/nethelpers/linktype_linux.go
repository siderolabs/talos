// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "golang.org/x/sys/unix"

//go:generate stringer -type=LinkType -linecomment -output linktype_string_linux.go

// LinkType is a link type.
type LinkType uint16

// MarshalYAML implements yaml.Marshaler.
func (typ LinkType) MarshalYAML() (interface{}, error) {
	return typ.String(), nil
}

// LinkType constants.
const (
	LinkNetrom            LinkType = unix.ARPHRD_NETROM             // netrom
	LinkEther             LinkType = unix.ARPHRD_ETHER              // ether
	LinkEether            LinkType = unix.ARPHRD_EETHER             // eether
	LinkAx25              LinkType = unix.ARPHRD_AX25               // ax25
	LinkPronet            LinkType = unix.ARPHRD_PRONET             // pronet
	LinkChaos             LinkType = unix.ARPHRD_CHAOS              // chaos
	LinkIee802            LinkType = unix.ARPHRD_IEEE802            // ieee802
	LinkArcnet            LinkType = unix.ARPHRD_ARCNET             // arcnet
	LinkAtalk             LinkType = unix.ARPHRD_APPLETLK           // atalk
	LinkDlci              LinkType = unix.ARPHRD_DLCI               // dlci
	LinkAtm               LinkType = unix.ARPHRD_ATM                // atm
	LinkMetricom          LinkType = unix.ARPHRD_METRICOM           // metricom
	LinkIeee1394          LinkType = unix.ARPHRD_IEEE1394           // ieee1394
	LinkEui64             LinkType = unix.ARPHRD_EUI64              // eui64
	LinkInfiniband        LinkType = unix.ARPHRD_INFINIBAND         // infiniband
	LinkSlip              LinkType = unix.ARPHRD_SLIP               // slip
	LinkCslip             LinkType = unix.ARPHRD_CSLIP              // cslip
	LinkSlip6             LinkType = unix.ARPHRD_SLIP6              // slip6
	LinkCslip6            LinkType = unix.ARPHRD_CSLIP6             // cslip6
	LinkRsrvd             LinkType = unix.ARPHRD_RSRVD              // rsrvd
	LinkAdapt             LinkType = unix.ARPHRD_ADAPT              // adapt
	LinkRose              LinkType = unix.ARPHRD_ROSE               // rose
	LinkX25               LinkType = unix.ARPHRD_X25                // x25
	LinkHwx25             LinkType = unix.ARPHRD_HWX25              // hwx25
	LinkCan               LinkType = unix.ARPHRD_CAN                // can
	LinkPpp               LinkType = unix.ARPHRD_PPP                // ppp
	LinkCisco             LinkType = unix.ARPHRD_CISCO              // cisco
	LinkHdlc              LinkType = unix.ARPHRD_HDLC               // hdlc
	LinkLapb              LinkType = unix.ARPHRD_LAPB               // lapb
	LinkDdcmp             LinkType = unix.ARPHRD_DDCMP              // ddcmp
	LinkRawhdlc           LinkType = unix.ARPHRD_RAWHDLC            // rawhdlc
	LinkTunnel            LinkType = unix.ARPHRD_TUNNEL             // ipip
	LinkTunnel6           LinkType = unix.ARPHRD_TUNNEL6            // tunnel6
	LinkFrad              LinkType = unix.ARPHRD_FRAD               // frad
	LinkSkip              LinkType = unix.ARPHRD_SKIP               // skip
	LinkLoopbck           LinkType = unix.ARPHRD_LOOPBACK           // loopback
	LinkLocaltlk          LinkType = unix.ARPHRD_LOCALTLK           // localtlk
	LinkFddi              LinkType = unix.ARPHRD_FDDI               // fddi
	LinkBif               LinkType = unix.ARPHRD_BIF                // bif
	LinkSit               LinkType = unix.ARPHRD_SIT                // sit
	LinkIpddp             LinkType = unix.ARPHRD_IPDDP              // ip/ddp
	LinkIpgre             LinkType = unix.ARPHRD_IPGRE              // gre
	LinkPimreg            LinkType = unix.ARPHRD_PIMREG             // pimreg
	LinkHippi             LinkType = unix.ARPHRD_HIPPI              // hippi
	LinkAsh               LinkType = unix.ARPHRD_ASH                // ash
	LinkEconet            LinkType = unix.ARPHRD_ECONET             // econet
	LinkIrda              LinkType = unix.ARPHRD_IRDA               // irda
	LinkFcpp              LinkType = unix.ARPHRD_FCPP               // fcpp
	LinkFcal              LinkType = unix.ARPHRD_FCAL               // fcal
	LinkFcpl              LinkType = unix.ARPHRD_FCPL               // fcpl
	LinkFcfabric          LinkType = unix.ARPHRD_FCFABRIC           // fcfb_0
	LinkFcfabric1         LinkType = unix.ARPHRD_FCFABRIC + 1       // fcfb_1
	LinkFcfabric2         LinkType = unix.ARPHRD_FCFABRIC + 2       // fcfb_2
	LinkFcfabric3         LinkType = unix.ARPHRD_FCFABRIC + 3       // fcfb_3
	LinkFcfabric4         LinkType = unix.ARPHRD_FCFABRIC + 4       // fcfb_4
	LinkFcfabric5         LinkType = unix.ARPHRD_FCFABRIC + 5       // fcfb_5
	LinkFcfabric6         LinkType = unix.ARPHRD_FCFABRIC + 6       // fcfb_6
	LinkFcfabric7         LinkType = unix.ARPHRD_FCFABRIC + 7       // fcfb_7
	LinkFcfabric8         LinkType = unix.ARPHRD_FCFABRIC + 8       // fcfb_8
	LinkFcfabric9         LinkType = unix.ARPHRD_FCFABRIC + 9       // fcfb_9
	LinkFcfabric10        LinkType = unix.ARPHRD_FCFABRIC + 10      // fcfb_10
	LinkFcfabric11        LinkType = unix.ARPHRD_FCFABRIC + 11      // fcfb_11
	LinkFcfabric12        LinkType = unix.ARPHRD_FCFABRIC + 12      // fcfb_12
	LinkIee802tr          LinkType = unix.ARPHRD_IEEE802_TR         // tr
	LinkIee80211          LinkType = unix.ARPHRD_IEEE80211          // ieee802.11
	LinkIee80211prism     LinkType = unix.ARPHRD_IEEE80211_PRISM    // ieee802.11_prism
	LinkIee80211Radiotap  LinkType = unix.ARPHRD_IEEE80211_RADIOTAP // ieee802.11_radiotap
	LinkIee8021154        LinkType = unix.ARPHRD_IEEE802154         // ieee802.15.4
	LinkIee8021154monitor LinkType = unix.ARPHRD_IEEE802154_MONITOR // ieee802.15.4_monitor
	LinkPhonet            LinkType = unix.ARPHRD_PHONET             // phonet
	LinkPhonetpipe        LinkType = unix.ARPHRD_PHONET_PIPE        // phonet_pipe
	LinkCaif              LinkType = unix.ARPHRD_CAIF               // caif
	LinkIP6gre            LinkType = unix.ARPHRD_IP6GRE             // ip6gre
	LinkNetlink           LinkType = unix.ARPHRD_NETLINK            // netlink
	Link6Lowpan           LinkType = unix.ARPHRD_6LOWPAN            // 6lowpan
	LinkVoid              LinkType = unix.ARPHRD_VOID               // void
	LinkNone              LinkType = unix.ARPHRD_NONE               // nohdr
)
