// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:golint,stylecheck,revive
package nic

import "fmt"

// https://elixir.bootlin.com/linux/latest/source/include/uapi/linux/if_link.h#L608
type BondSetting uint16

const (
	IFLA_BOND_UNSPEC BondSetting = iota
	IFLA_BOND_MODE
	IFLA_BOND_ACTIVE_SLAVE
	IFLA_BOND_MIIMON
	IFLA_BOND_UPDELAY
	IFLA_BOND_DOWNDELAY
	IFLA_BOND_USE_CARRIER
	IFLA_BOND_ARP_INTERVAL
	IFLA_BOND_ARP_IP_TARGET
	IFLA_BOND_ARP_VALIDATE
	IFLA_BOND_ARP_ALL_TARGETS
	IFLA_BOND_PRIMARY
	IFLA_BOND_PRIMARY_RESELECT
	IFLA_BOND_FAIL_OVER_MAC
	IFLA_BOND_XMIT_HASH_POLICY
	IFLA_BOND_RESEND_IGMP
	IFLA_BOND_NUM_PEER_NOTIF
	IFLA_BOND_ALL_SLAVES_ACTIVE
	IFLA_BOND_MIN_LINKS
	IFLA_BOND_LP_INTERVAL
	IFLA_BOND_PACKETS_PER_SLAVE
	IFLA_BOND_AD_LACP_RATE
	IFLA_BOND_AD_SELECT
	IFLA_BOND_AD_INFO
	IFLA_BOND_AD_ACTOR_SYS_PRIO
	IFLA_BOND_AD_USER_PORT_KEY
	IFLA_BOND_AD_ACTOR_SYSTEM
	IFLA_BOND_TLB_DYNAMIC_LB
	IFLA_BOND_PEER_NOTIF_DELAY
)

func (b BondSetting) String() string {
	return [...]string{
		"unspec",
		"mode",
		"active slave",
		"miimon",
		"updelay",
		"downdelay",
		"use carrier",
		"arp interval",
		"arp ip target",
		"arp validate",
		"arp all targets",
		"primary",
		"primary reselect",
		"fail over mac",
		"xmit hash policy",
		"resend igmp",
		"num peer notif",
		"all slaves active",
		"min links",
		"lp interval",
		"packets per slave",
		"ad lacp rate",
		"ad select",
		"ad innfo",
		"ad actor sys prio",
		"ad user port key",
		"ad actor system",
		"tlb dynamic lb",
		"peer notif delay",
	}[int(b)]
}

// https://elixir.bootlin.com/linux/latest/source/include/uapi/linux/if_bonding.h
type BondMode uint8

const (
	BOND_MODE_ROUNDROBIN BondMode = iota
	BOND_MODE_ACTIVEBACKUP
	BOND_MODE_XOR
	BOND_MODE_BROADCAST
	BOND_MODE_8023AD
	BOND_MODE_TLB
	BOND_MODE_ALB
)

func (b BondMode) String() string {
	return [...]string{"balance-rr", "active-backup", "balance-xor", "broadcast", "802.3ad", "balance-tlb", "balance-alb"}[int(b)]
}

func BondModeByName(mode string) (bm BondMode, err error) {
	switch mode {
	case "balance-rr":
		bm = BOND_MODE_ROUNDROBIN
	case "active-backup":
		bm = BOND_MODE_ACTIVEBACKUP
	case "balance-xor":
		bm = BOND_MODE_XOR
	case "broadcast":
		bm = BOND_MODE_BROADCAST
	case "802.3ad":
		bm = BOND_MODE_8023AD
	case "balance-tlb":
		bm = BOND_MODE_TLB
	case "balance-alb":
		bm = BOND_MODE_ALB
	default:
		err = fmt.Errorf("invalid bond type %s", mode)
	}

	return bm, err
}

type BondXmitHashPolicy uint8

const (
	BOND_XMIT_POLICY_LAYER2 BondXmitHashPolicy = iota
	BOND_XMIT_POLICY_LAYER34
	BOND_XMIT_POLICY_LAYER23
	BOND_XMIT_POLICY_ENCAP23
	BOND_XMIT_POLICY_ENCAP34
)

func (b BondXmitHashPolicy) String() string {
	return [...]string{"layer2", "layer3+4", "layer2+3", "encap2+3", "encap3+4"}[int(b)]
}

func BondXmitHashPolicyByName(policy string) (xmit BondXmitHashPolicy, err error) {
	switch policy {
	case "layer2":
		xmit = BOND_XMIT_POLICY_LAYER2
	case "layer3+4":
		xmit = BOND_XMIT_POLICY_LAYER34
	case "layer2+3":
		xmit = BOND_XMIT_POLICY_LAYER23
	case "encap2+3":
		xmit = BOND_XMIT_POLICY_ENCAP23
	case "encap3+4":
		xmit = BOND_XMIT_POLICY_ENCAP34
	default:
		err = fmt.Errorf("invalid xmit hash policy %v", xmit)
	}

	return xmit, err
}

type LACPRate uint8

const (
	LACP_RATE_SLOW LACPRate = iota
	LACP_RATE_FAST
)

func (l LACPRate) String() string {
	return [...]string{"slow", "fast"}[l]
}

func LACPRateByName(mode string) (rate LACPRate, err error) {
	switch mode {
	case "slow":
		rate = LACP_RATE_SLOW
	case "fast":
		rate = LACP_RATE_FAST
	default:
		err = fmt.Errorf("invalid lacp rate %v", mode)
	}

	return rate, err
}

type ADSelect uint8

const (
	AD_SELECT_STABLE ADSelect = iota
	AD_SELECT_BANDWIDTH
	AD_SELECT_COUNT
)

func (a ADSelect) String() string {
	return [...]string{"stable", "bandwidth", "count"}[a]
}

func ADSelectByName(sel string) (adsel ADSelect, err error) {
	switch sel {
	case "stable":
		adsel = AD_SELECT_STABLE
	case "bandwidth":
		adsel = AD_SELECT_BANDWIDTH
	case "count":
		adsel = AD_SELECT_COUNT
	default:
		err = fmt.Errorf("invalid ad_select mode %v", sel)
	}

	return adsel, err
}

type ARPValidate uint32

const (
	ARP_VALIDATE_NONE ARPValidate = iota
	ARP_VALIDATE_ACTIVE
	ARP_VALIDATE_BACKUP
	ARP_VALIDATE_ALL
)

func (a ARPValidate) String() string {
	return [...]string{"none", "active", "backup", "all"}[a]
}

func ARPValidateByName(a string) (arpv ARPValidate, err error) {
	switch a {
	case "none":
		arpv = ARP_VALIDATE_NONE
	case "active":
		arpv = ARP_VALIDATE_ACTIVE
	case "backup":
		arpv = ARP_VALIDATE_BACKUP
	case "all":
		arpv = ARP_VALIDATE_ALL
	default:
		err = fmt.Errorf("invalid arp_validate mode %v", a)
	}

	return arpv, err
}

type ARPAllTargets uint32

const (
	ARP_ALL_TARGETS_ANY ARPAllTargets = iota
	ARP_ALL_TARGETS_ALL
)

func (a ARPAllTargets) String() string {
	return [...]string{"any", "all"}[a]
}

func ARPAllTargetsByName(a string) (arpa ARPAllTargets, err error) {
	switch a {
	case "any":
		arpa = ARP_ALL_TARGETS_ANY
	case "all":
		arpa = ARP_ALL_TARGETS_ALL
	default:
		err = fmt.Errorf("invalid arp_all_targets mode %v", a)
	}

	return arpa, err
}

type PrimaryReselect uint8

const (
	PRIMARY_RESELECT_ALWAYS PrimaryReselect = iota
	PRIMARY_RESELECT_BETTER
	PRIMARY_RESELECT_FAILURE
)

func (p PrimaryReselect) String() string {
	return [...]string{"always", "better", "failure"}[p]
}

func PrimaryReselectByName(p string) (pr PrimaryReselect, err error) {
	switch p {
	case "always":
		pr = PRIMARY_RESELECT_ALWAYS
	case "better":
		pr = PRIMARY_RESELECT_BETTER
	case "failure":
		pr = PRIMARY_RESELECT_FAILURE
	default:
		err = fmt.Errorf("invalid primary_reselect mode %v", p)
	}

	return pr, err
}

type FailOverMAC uint8

const (
	FAIL_OVER_MAC_NONE FailOverMAC = iota
	FAIL_OVER_MAC_ACTIVE
	FAIL_OVER_MAC_FOLLOW
)

func FailOverMACByName(f string) (fo FailOverMAC, err error) {
	switch f {
	case "none":
		fo = FAIL_OVER_MAC_NONE
	case "active":
		fo = FAIL_OVER_MAC_ACTIVE
	case "follow":
		fo = FAIL_OVER_MAC_FOLLOW
	default:
		err = fmt.Errorf("invalid fail_over_mac value %v", f)
	}

	return fo, err
}

const (
	IFLA_VLAN_UNSPEC = iota
	IFLA_VLAN_ID
	IFLA_VLAN_FLAGS
	IFLA_VLAN_EGRESS_QOS
	IFLA_VLAN_INGRESS_QOS
	IFLA_VLAN_PROTOCOL
	IFLA_VLAN_MAX = IFLA_VLAN_PROTOCOL
)

// VlanProtocol possible values.
const (
	VLAN_PROTOCOL_UNKNOWN = 0
	VLAN_PROTOCOL_8021Q   = 0x8100
	VLAN_PROTOCOL_8021AD  = 0x88A8
)
