// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"net/netip"

	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// BondKind is a Bond config document kind.
const BondKind = "BondConfig"

func init() {
	registry.Register(BondKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &BondConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkBondConfig   = &BondConfigV1Alpha1{}
	_ config.ConflictingDocument = &BondConfigV1Alpha1{}
	_ config.NamedDocument       = &BondConfigV1Alpha1{}
	_ config.Validator           = &BondConfigV1Alpha1{}
)

// BondConfigV1Alpha1 is a config document to create a bond (link aggregation) over a set of links.
//
//	examples:
//	  - value: exampleBondConfigV1Alpha1()
//	alias: BondConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/BondConfig
type BondConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the bond link (interface) to be created.
	//
	//   examples:
	//    - value: >
	//       "bond.ext"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Override the hardware (MAC) address of the link.
	//
	//   examples:
	//    - value: >
	//       nethelpers.HardwareAddr{0x2e, 0x3c, 0x4d, 0x5e, 0x6f, 0x70}
	//   schema:
	//     type: string
	//     pattern: ^[0-9a-f:]+$
	HardwareAddressConfig nethelpers.HardwareAddr `yaml:"hardwareAddr,omitempty"`
	//   description: |
	//     Names of the links (interfaces) on which the bond will be created.
	//     Link aliases can be used here as well.
	//   examples:
	//    - value: >
	//       []string{"enp0s3", "enp0s8"}
	//   schemaRequired: true
	BondLinks []string `yaml:"links,omitempty"`
	//   description: |
	//     Bond mode.
	//
	//   examples:
	//    - value: >
	//       "802.3ad"
	//   values:
	//     - "balance-rr"
	//     - "active-backup"
	//     - "balance-xor"
	//     - "broadcast"
	//     - "802.3ad"
	//     - "balance-tlb"
	//     - "balance-alb"
	//   schemaRequired: true
	BondMode *nethelpers.BondMode `yaml:"bondMode,omitempty"`
	//   description: |
	//     Link monitoring frequency in milliseconds.
	//
	//   examples:
	//    - value: >
	//       200
	BondMIIMon *uint32 `yaml:"miimon,omitempty"`
	//   description: |
	//     The time, in milliseconds, to wait before enabling a slave after a link recovery has been detected.
	//
	//   examples:
	//    - value: >
	//       300
	BondUpDelay *uint32 `yaml:"updelay,omitempty"`
	//   description: |
	//     The time, in milliseconds, to wait before disabling a slave after a link failure has been detected.
	//
	//   examples:
	//    - value: >
	//       100
	BondDownDelay *uint32 `yaml:"downdelay,omitempty"`
	//   description: |
	//     Specifies whether or not miimon should use MII or ETHTOOL.
	BondUseCarrier *bool `yaml:"useCarrier,omitempty"`
	//   description: |
	//     Selects the transmit hash policy to use for slave selection.
	//   examples:
	//    - value: >
	//       "layer2"
	//   values:
	//     - "layer2"
	//     - "layer3+4"
	//     - "layer2+3"
	//     - "encap2+3"
	//     - "encap3+4"
	BondXmitHashPolicy *nethelpers.BondXmitHashPolicy `yaml:"xmitHashPolicy,omitempty"`
	//   description: |
	//    ARP link monitoring frequency in milliseconds.
	//   examples:
	//    - value: >
	//       1000
	BondARPInterval *uint32 `yaml:"arpInterval,omitempty"`
	//   description: |
	//     The list of IPv4 addresses to use for ARP link monitoring when arpInterval is set.
	//     Maximum of 16 targets are supported.
	//   examples:
	//    - value: >
	//       []netip.Addr{netip.MustParseAddr("10.15.0.1")}
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+$
	BondARPIPTargets []netip.Addr `yaml:"arpIpTargets,omitempty"`
	//   description: |
	//     The list of IPv6 addresses to use for NS link monitoring when arpInterval is set.
	//     Maximum of 16 targets are supported.
	//   examples:
	//    - value: >
	//       []netip.Addr{netip.MustParseAddr("fd00::1")}
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+$
	BondNSIP6Targets []netip.Addr `yaml:"nsIp6Targets,omitempty"`
	//   description: |
	//     Specifies whether or not ARP probes and replies should be validated.
	//   examples:
	//    - value: >
	//       "active"
	//   values:
	//     - "none"
	//     - "active"
	//     - "backup"
	//     - "all"
	//     - "filter"
	//     - "filter-active"
	//     - "filter-backup"
	BondARPValidate *nethelpers.ARPValidate `yaml:"arpValidate,omitempty"`
	//   description: |
	//     Specifies whether ARP probes should be sent to any or all targets.
	//   examples:
	//    - value: >
	//       "all"
	//   values:
	//     - "any"
	//     - "all"
	BondARPAllTargets *nethelpers.ARPAllTargets `yaml:"arpAllTargets,omitempty"`
	//   description: |
	//     LACPDU frames periodic transmission rate.
	//   examples:
	//    - value: >
	//       "fast"
	//   values:
	//     - "slow"
	//     - "fast"
	BondLACPRate *nethelpers.LACPRate `yaml:"lacpRate,omitempty"`
	//   description: |
	//     Specifies whether active-backup mode should set all slaves to the same MAC address
	//     at enslavement, when enabled, or perform special handling.
	//   examples:
	//    - value: >
	//       "active"
	//   values:
	//     - "none"
	//     - "active"
	//     - "follow"
	BondFailOverMAC *nethelpers.FailOverMAC `yaml:"failOverMac,omitempty"`
	//   description: |
	//     Aggregate selection policy for 802.3ad.
	//   examples:
	//    - value: >
	//       "stable"
	//   values:
	//     - "stable"
	//     - "bandwidth"
	//     - "count"
	BondADSelect *nethelpers.ADSelect `yaml:"adSelect,omitempty"`
	//   description: |
	//     Actor system priority for 802.3ad.
	//
	//   examples:
	//    - value: >
	//       65535
	BondADActorSysPrio *uint16 `yaml:"adActorSysPrio,omitempty"`
	//   description: |
	//     User port key (upper 10 bits) for 802.3ad.
	//
	//   examples:
	//    - value: >
	//       0
	BondADUserPortKey *uint16 `yaml:"adUserPortKey,omitempty"`
	//   description: |
	//     Whether to send LACPDU frames periodically.
	//   examples:
	//    - value: >
	//       "on"
	//   values:
	//     - "on"
	//     - "off"
	BondADLACPActive *nethelpers.ADLACPActive `yaml:"adLACPActive,omitempty"`
	//   description: |
	//     Policy under which the primary slave should be reselected.
	//   examples:
	//    - value: >
	//       "always"
	//   values:
	//     - "always"
	//     - "better"
	//     - "failure"
	BondPrimaryReselect *nethelpers.PrimaryReselect `yaml:"primaryReselect,omitempty"`
	//   description: |
	//     The number of times IGMP packets should be resent.
	BondResendIGMP *uint32 `yaml:"resendIGMP,omitempty"`
	//   description: |
	//     The minimum number of active links required for the bond to be considered active.
	BondMinLinks *uint32 `yaml:"minLinks,omitempty"`
	//   description: |
	//     The number of seconds between instances where the bonding driver sends learning packets to each slave's peer switch.
	BondLPInterval *uint32 `yaml:"lpInterval,omitempty"`
	//   description: |
	//     The number of packets to transmit through a slave before moving to the next one.
	BondPacketsPerSlave *uint32 `yaml:"packetsPerSlave,omitempty"`
	//   description: |
	//     The number of peer notifications (gratuitous ARPs and unsolicited IPv6 Neighbor Advertisements)
	//     to be issued after a failover event.
	BondNumPeerNotif *uint8 `yaml:"numPeerNotif,omitempty"`
	//   description: |
	//     Whether dynamic shuffling of flows is enabled in tlb or alb mode.
	//   examples:
	//    - value: >
	//       1
	BondTLBDynamicLB *uint8 `yaml:"tlbLogicalLb,omitempty"`
	//   description: |
	//     Whether duplicate frames (received on inactive ports) should be dropped (0) or delivered (1).
	//   examples:
	//    - value: >
	//       0
	BondAllSlavesActive *uint8 `yaml:"allSlavesActive,omitempty"`
	//   description: |
	//     The delay, in milliseconds, between each peer notification.
	BondPeerNotifyDelay *uint32 `yaml:"peerNotifDelay,omitempty"`
	//   description: |
	//     The number of arpInterval monitor checks that must fail in order for an interface to be marked down by the ARP monitor.
	BondMissedMax *uint8 `yaml:"missedMax,omitempty"`

	//nolint:embeddedstructfieldcheck
	CommonLinkConfig `yaml:",inline"`
}

// NewBondConfigV1Alpha1 creates a new BondConfig config document.
func NewBondConfigV1Alpha1(name string) *BondConfigV1Alpha1 {
	return &BondConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       BondKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleBondConfigV1Alpha1() *BondConfigV1Alpha1 {
	cfg := NewBondConfigV1Alpha1("bond.int")
	cfg.BondLinks = []string{"enp1s2", "enp1s2"}
	cfg.BondMode = new(nethelpers.BondMode8023AD)
	cfg.BondXmitHashPolicy = new(nethelpers.BondXmitPolicyLayer34)
	cfg.BondLACPRate = new(nethelpers.LACPRateSlow)
	cfg.BondMIIMon = new(uint32(100))
	cfg.BondUpDelay = new(uint32(200))
	cfg.BondDownDelay = new(uint32(200))
	cfg.BondResendIGMP = new(uint32(1))
	cfg.BondPacketsPerSlave = new(uint32(1))
	cfg.BondADActorSysPrio = new(uint16(65535))

	cfg.LinkAddresses = []AddressConfig{
		{
			AddressAddress: netip.MustParsePrefix("10.15.0.3/16"),
		},
	}
	cfg.LinkRoutes = []RouteConfig{
		{
			RouteDestination: Prefix{netip.MustParsePrefix("10.0.0.0/8")},
			RouteGateway:     Addr{netip.MustParseAddr("10.15.0.1")},
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *BondConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *BondConfigV1Alpha1) Name() string {
	return s.MetaName
}

// BondConfig implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) BondConfig() {}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *BondConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingLinkKinds(BondKind)
}

// Validate implements config.Validator interface.
func (s *BondConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if len(s.BondLinks) == 0 {
		errs = errors.Join(errs, errors.New("at least one link must be specified"))
	}

	if s.BondMode == nil {
		errs = errors.Join(errs, errors.New("bond mode must be specified"))
	} else if *s.BondMode == nethelpers.BondMode8023AD {
		warnings = append(warnings, s.validateFor8023AD()...)
	}

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}

func (s *BondConfigV1Alpha1) validateFor8023AD() []string {
	const warn = " was not specified for 802.3ad bond"

	var warnings []string

	if s.BondMIIMon == nil {
		warnings = append(warnings, "miimon"+warn)
	}

	if s.BondUpDelay == nil {
		warnings = append(warnings, "updelay"+warn)
	}

	if s.BondDownDelay == nil {
		warnings = append(warnings, "downdelay"+warn)
	}

	return warnings
}

// Links implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) Links() []string {
	return s.BondLinks
}

// Mode implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) Mode() nethelpers.BondMode {
	return pointer.SafeDeref(s.BondMode)
}

// MIIMon implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) MIIMon() optional.Optional[uint32] {
	if s.BondMIIMon == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondMIIMon)
}

// UpDelay implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) UpDelay() optional.Optional[uint32] {
	if s.BondUpDelay == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondUpDelay)
}

// DownDelay implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) DownDelay() optional.Optional[uint32] {
	if s.BondDownDelay == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondDownDelay)
}

// UseCarrier implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) UseCarrier() optional.Optional[bool] {
	if s.BondUseCarrier == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.BondUseCarrier)
}

// XmitHashPolicy implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) XmitHashPolicy() optional.Optional[nethelpers.BondXmitHashPolicy] {
	if s.BondXmitHashPolicy == nil {
		return optional.None[nethelpers.BondXmitHashPolicy]()
	}

	return optional.Some(*s.BondXmitHashPolicy)
}

// ARPInterval implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ARPInterval() optional.Optional[uint32] {
	if s.BondARPInterval == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondARPInterval)
}

// ARPIPTargets implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ARPIPTargets() []netip.Addr {
	return s.BondARPIPTargets
}

// NSIP6Targets implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) NSIP6Targets() []netip.Addr {
	return s.BondNSIP6Targets
}

// ARPValidate implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ARPValidate() optional.Optional[nethelpers.ARPValidate] {
	if s.BondARPValidate == nil {
		return optional.None[nethelpers.ARPValidate]()
	}

	return optional.Some(*s.BondARPValidate)
}

// ARPAllTargets implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ARPAllTargets() optional.Optional[nethelpers.ARPAllTargets] {
	if s.BondARPAllTargets == nil {
		return optional.None[nethelpers.ARPAllTargets]()
	}

	return optional.Some(*s.BondARPAllTargets)
}

// LACPRate implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) LACPRate() optional.Optional[nethelpers.LACPRate] {
	if s.BondLACPRate == nil {
		return optional.None[nethelpers.LACPRate]()
	}

	return optional.Some(*s.BondLACPRate)
}

// FailOverMAC implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) FailOverMAC() optional.Optional[nethelpers.FailOverMAC] {
	if s.BondFailOverMAC == nil {
		return optional.None[nethelpers.FailOverMAC]()
	}

	return optional.Some(*s.BondFailOverMAC)
}

// ADSelect implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ADSelect() optional.Optional[nethelpers.ADSelect] {
	if s.BondADSelect == nil {
		return optional.None[nethelpers.ADSelect]()
	}

	return optional.Some(*s.BondADSelect)
}

// ADActorSysPrio implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ADActorSysPrio() optional.Optional[uint16] {
	if s.BondADActorSysPrio == nil {
		return optional.None[uint16]()
	}

	return optional.Some(*s.BondADActorSysPrio)
}

// ADUserPortKey implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ADUserPortKey() optional.Optional[uint16] {
	if s.BondADUserPortKey == nil {
		return optional.None[uint16]()
	}

	return optional.Some(*s.BondADUserPortKey)
}

// ADLACPActive implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ADLACPActive() optional.Optional[nethelpers.ADLACPActive] {
	if s.BondADLACPActive == nil {
		return optional.None[nethelpers.ADLACPActive]()
	}

	return optional.Some(*s.BondADLACPActive)
}

// PrimaryReselect implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) PrimaryReselect() optional.Optional[nethelpers.PrimaryReselect] {
	if s.BondPrimaryReselect == nil {
		return optional.None[nethelpers.PrimaryReselect]()
	}

	return optional.Some(*s.BondPrimaryReselect)
}

// ResendIGMP implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) ResendIGMP() optional.Optional[uint32] {
	if s.BondResendIGMP == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondResendIGMP)
}

// MinLinks implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) MinLinks() optional.Optional[uint32] {
	if s.BondMinLinks == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondMinLinks)
}

// LPInterval implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) LPInterval() optional.Optional[uint32] {
	if s.BondLPInterval == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondLPInterval)
}

// PacketsPerSlave implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) PacketsPerSlave() optional.Optional[uint32] {
	if s.BondPacketsPerSlave == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondPacketsPerSlave)
}

// NumPeerNotif implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) NumPeerNotif() optional.Optional[uint8] {
	if s.BondNumPeerNotif == nil {
		return optional.None[uint8]()
	}

	return optional.Some(*s.BondNumPeerNotif)
}

// TLBDynamicLB implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) TLBDynamicLB() optional.Optional[uint8] {
	if s.BondTLBDynamicLB == nil {
		return optional.None[uint8]()
	}

	return optional.Some(*s.BondTLBDynamicLB)
}

// AllSlavesActive implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) AllSlavesActive() optional.Optional[uint8] {
	if s.BondAllSlavesActive == nil {
		return optional.None[uint8]()
	}

	return optional.Some(*s.BondAllSlavesActive)
}

// PeerNotifyDelay implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) PeerNotifyDelay() optional.Optional[uint32] {
	if s.BondPeerNotifyDelay == nil {
		return optional.None[uint32]()
	}

	return optional.Some(*s.BondPeerNotifyDelay)
}

// MissedMax implements NetworkBondConfig interface.
func (s *BondConfigV1Alpha1) MissedMax() optional.Optional[uint8] {
	if s.BondMissedMax == nil {
		return optional.None[uint8]()
	}

	return optional.Some(*s.BondMissedMax)
}

// HardwareAddress implements NetworkDummyLinkConfig interface.
func (s *BondConfigV1Alpha1) HardwareAddress() optional.Optional[nethelpers.HardwareAddr] {
	if len(s.HardwareAddressConfig) == 0 {
		return optional.None[nethelpers.HardwareAddr]()
	}

	return optional.Some(s.HardwareAddressConfig)
}
