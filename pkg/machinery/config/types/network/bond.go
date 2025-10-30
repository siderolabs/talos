// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"net/netip"

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
	//     Names of the parent links (interfaces) on which the bond link will be created.
	//     Link aliases can be used here as well.
	//   examples:
	//    - value: >
	//       []string{"enp0s3", "enp0s8"}
	//   schemaRequired: true
	ParentLinks []string `yaml:"parentLinks,omitempty"`
	//   description: |
	//     Bond ID to be used for the Bond link.
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
	//     Device index specifying which slave is the primary device.
	BondPrimaryIndex *uint32 `yaml:"primary,omitempty"`
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
	BondNumPeerNotif *uint32 `yaml:"numPeerNotif,omitempty"`
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
	BondPeerNotifDelay *uint32 `yaml:"peerNotifDelay,omitempty"`
	//   description: |
	//     The number of arpInterval monitor checks that must fail in order for an interface to be marked down by the ARP monitor.
	BondMissedMax *uint32 `yaml:"missedMax,omitempty"`

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
	cfg.ParentLinks = []string{"enp1s2", "enp1s2"}
	cfg.BondMode = pointer.To(nethelpers.BondMode8023AD)
	cfg.BondXmitHashPolicy = pointer.To(nethelpers.BondXmitPolicyLayer34)
	cfg.BondLACPRate = pointer.To(nethelpers.LACPRateSlow)
	cfg.BondMIIMon = pointer.To(uint32(100))
	cfg.BondUpDelay = pointer.To(uint32(200))
	cfg.BondDownDelay = pointer.To(uint32(200))
	cfg.BondResendIGMP = pointer.To(uint32(1))
	cfg.BondPacketsPerSlave = pointer.To(uint32(1))
	cfg.BondADActorSysPrio = pointer.To(uint16(65535))

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

	extraWarnings, extraErrs := s.CommonLinkConfig.Validate()
	errs, warnings = errors.Join(errs, extraErrs), append(warnings, extraWarnings...)

	return warnings, errs
}
