// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate enumer -type=LinkType -linecomment -text

// LinkType is a link type.
type LinkType uint16

// LinkType constants.
const (
	LinkNetrom            LinkType = 0                 // netrom
	LinkEther             LinkType = 1                 // ether
	LinkEether            LinkType = 2                 // eether
	LinkAx25              LinkType = 3                 // ax25
	LinkPronet            LinkType = 4                 // pronet
	LinkChaos             LinkType = 5                 // chaos
	LinkIee802            LinkType = 6                 // ieee802
	LinkArcnet            LinkType = 7                 // arcnet
	LinkAtalk             LinkType = 8                 // atalk
	LinkDlci              LinkType = 15                // dlci
	LinkAtm               LinkType = 19                // atm
	LinkMetricom          LinkType = 23                // metricom
	LinkIeee1394          LinkType = 24                // ieee1394
	LinkEui64             LinkType = 27                // eui64
	LinkInfiniband        LinkType = 32                // infiniband
	LinkSlip              LinkType = 256               // slip
	LinkCslip             LinkType = 257               // cslip
	LinkSlip6             LinkType = 258               // slip6
	LinkCslip6            LinkType = 259               // cslip6
	LinkRsrvd             LinkType = 260               // rsrvd
	LinkAdapt             LinkType = 264               // adapt
	LinkRose              LinkType = 270               // rose
	LinkX25               LinkType = 271               // x25
	LinkHwx25             LinkType = 272               // hwx25
	LinkCan               LinkType = 280               // can
	LinkPpp               LinkType = 512               // ppp
	LinkCisco             LinkType = 513               // cisco
	LinkHdlc              LinkType = 513               // hdlc
	LinkLapb              LinkType = 516               // lapb
	LinkDdcmp             LinkType = 517               // ddcmp
	LinkRawhdlc           LinkType = 518               // rawhdlc
	LinkTunnel            LinkType = 768               // ipip
	LinkTunnel6           LinkType = 769               // tunnel6
	LinkFrad              LinkType = 770               // frad
	LinkSkip              LinkType = 771               // skip
	LinkLoopbck           LinkType = 772               // loopback
	LinkLocaltlk          LinkType = 773               // localtlk
	LinkFddi              LinkType = 774               // fddi
	LinkBif               LinkType = 775               // bif
	LinkSit               LinkType = 776               // sit
	LinkIpddp             LinkType = 777               // ip/ddp
	LinkIpgre             LinkType = 778               // gre
	LinkPimreg            LinkType = 779               // pimreg
	LinkHippi             LinkType = 780               // hippi
	LinkAsh               LinkType = 781               // ash
	LinkEconet            LinkType = 782               // econet
	LinkIrda              LinkType = 783               // irda
	LinkFcpp              LinkType = 784               // fcpp
	LinkFcal              LinkType = 785               // fcal
	LinkFcpl              LinkType = 786               // fcpl
	LinkFcfabric          LinkType = 787               // fcfb_0
	LinkFcfabric1         LinkType = LinkFcfabric + 1  // fcfb_1
	LinkFcfabric2         LinkType = LinkFcfabric + 2  // fcfb_2
	LinkFcfabric3         LinkType = LinkFcfabric + 3  // fcfb_3
	LinkFcfabric4         LinkType = LinkFcfabric + 4  // fcfb_4
	LinkFcfabric5         LinkType = LinkFcfabric + 5  // fcfb_5
	LinkFcfabric6         LinkType = LinkFcfabric + 6  // fcfb_6
	LinkFcfabric7         LinkType = LinkFcfabric + 7  // fcfb_7
	LinkFcfabric8         LinkType = LinkFcfabric + 8  // fcfb_8
	LinkFcfabric9         LinkType = LinkFcfabric + 9  // fcfb_9
	LinkFcfabric10        LinkType = LinkFcfabric + 10 // fcfb_10
	LinkFcfabric11        LinkType = LinkFcfabric + 11 // fcfb_11
	LinkFcfabric12        LinkType = LinkFcfabric + 12 // fcfb_12
	LinkIee802tr          LinkType = 800               // tr
	LinkIee80211          LinkType = 801               // ieee802.11
	LinkIee80211prism     LinkType = 802               // ieee802.11_prism
	LinkIee80211Radiotap  LinkType = 803               // ieee802.11_radiotap
	LinkIee8021154        LinkType = 804               // ieee802.15.4
	LinkIee8021154monitor LinkType = 805               // ieee802.15.4_monitor
	LinkPhonet            LinkType = 820               // phonet
	LinkPhonetpipe        LinkType = 821               // phonet_pipe
	LinkCaif              LinkType = 822               // caif
	LinkIP6gre            LinkType = 823               // ip6gre
	LinkNetlink           LinkType = 824               // netlink
	Link6Lowpan           LinkType = 825               // 6lowpan
	LinkVoid              LinkType = 65535             // void
	LinkNone              LinkType = 65534             // nohdr
)
