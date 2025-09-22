// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
)

// EthernetKind is a Ethernet config document kind.
const EthernetKind = "EthernetConfig"

func init() {
	registry.Register(EthernetKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &EthernetConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.EthernetConfig = &EthernetConfigV1Alpha1{}
	_ config.NamedDocument  = &EthernetConfigV1Alpha1{}
	_ config.Validator      = &EthernetConfigV1Alpha1{}
)

// EthernetConfigV1Alpha1 is a config document to configure Ethernet interfaces.
//
//	examples:
//	  - value: exampleEthernetConfigV1Alpha1()
//	alias: EthernetConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/EthernetConfig
type EthernetConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the link (interface).
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Configuration for Ethernet features.
	//
	//     Set of features available and whether they can be enabled or disabled is driver specific.
	//     Use `talosctl get ethernetstatus <link> -o yaml` to get the list of available features and
	//     their current status.
	FeaturesConfig map[string]bool `yaml:"features,omitempty"`
	//   description: |
	//     Configuration for Ethernet link rings.
	//
	//     This is similar to `ethtool -G` command.
	RingsConfig *EthernetRingsConfig `yaml:"rings,omitempty"`
	//   description: |
	//     Configuration for Ethernet link channels.
	//
	//     This is similar to `ethtool -L` command.
	ChannelsConfig *EthernetChannelsConfig `yaml:"channels,omitempty"`
	//   description: |
	//     Wake-on-LAN modes to enable.
	//
	//     If this field is omitted, Wake-on-LAN configuration is not changed.
	//     An empty list disables Wake-on-LAN.
	//
	//     This is similar to `ethtool -s <link> wol <options>` command.
	//   values:
	//     - "phy"
	//     - "unicast"
	//     - "multicast"
	//     - "broadcast"
	//     - "arp"
	//     - "magic"
	//     - "magicsecure"
	//     - "filter"
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//   examples:
	//    - value: >
	//       []nethelpers.WOLMode{nethelpers.WOLModeUnicast, nethelpers.WOLModeMagic}
	WakeOnLANConfig []nethelpers.WOLMode `yaml:"wakeOnLan,omitempty"`
}

// EthernetRingsConfig is a configuration for Ethernet link rings.
type EthernetRingsConfig struct {
	//   description: |
	//     Number of RX rings.
	RX *uint32 `yaml:"rx,omitempty"`
	//   description: |
	//     Number of TX rings.
	TX *uint32 `yaml:"tx,omitempty"`
	//   description: |
	//     Number of RX mini rings.
	RXMini *uint32 `yaml:"rx-mini,omitempty"`
	//  description: |
	//    Number of RX jumbo rings.
	RXJumbo *uint32 `yaml:"rx-jumbo,omitempty"`
	//   description: |
	//     RX buffer length.
	RXBufLen *uint32 `yaml:"rx-buf-len,omitempty"`
	//   description: |
	//     CQE size.
	CQESize *uint32 `yaml:"cqe-size,omitempty"`
	//   description: |
	//     TX push enabled.
	TXPush *bool `yaml:"tx-push,omitempty"`
	//  description: |
	//    RX push enabled.
	RXPush *bool `yaml:"rx-push,omitempty"`
	//  description: |
	//    TX push buffer length.
	TXPushBufLen *uint32 `yaml:"tx-push-buf-len,omitempty"`
	//  description: |
	//    TCP data split enabled.
	TCPDataSplit *bool `yaml:"tcp-data-split,omitempty"`
}

// EthernetChannelsConfig is a configuration for Ethernet link channels.
type EthernetChannelsConfig struct {
	//   description: |
	//     Number of RX channels.
	RX *uint32 `yaml:"rx,omitempty"`
	//   description: |
	//     Number of TX channels.
	TX *uint32 `yaml:"tx,omitempty"`
	//   description: |
	//     Number of other channels.
	Other *uint32 `yaml:"other,omitempty"`
	//   description: |
	//     Number of combined channels.
	Combined *uint32 `yaml:"combined,omitempty"`
}

// NewEthernetConfigV1Alpha1 creates a new EthernetConfig config document.
func NewEthernetConfigV1Alpha1(name string) *EthernetConfigV1Alpha1 {
	return &EthernetConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       EthernetKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleEthernetConfigV1Alpha1() *EthernetConfigV1Alpha1 {
	cfg := NewEthernetConfigV1Alpha1("enp0s2")
	cfg.RingsConfig = &EthernetRingsConfig{
		RX: pointer.To[uint32](256),
	}
	cfg.FeaturesConfig = map[string]bool{
		"tx-tcp-segmentation": false,
	}
	cfg.ChannelsConfig = &EthernetChannelsConfig{
		RX: pointer.To[uint32](4),
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *EthernetConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *EthernetConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Rings implements config.EthernetConfig interface.
func (s *EthernetConfigV1Alpha1) Rings() config.EthernetRingsConfig {
	return config.EthernetRingsConfig(pointer.SafeDeref(s.RingsConfig))
}

// Channels implements config.EthernetConfig interface.
func (s *EthernetConfigV1Alpha1) Channels() config.EthernetChannelsConfig {
	return config.EthernetChannelsConfig(pointer.SafeDeref(s.ChannelsConfig))
}

// Features implements config.EthernetConfig interface.
func (s *EthernetConfigV1Alpha1) Features() map[string]bool {
	return s.FeaturesConfig
}

// WakeOnLAN implements config.EthernetConfig interface.
func (s *EthernetConfigV1Alpha1) WakeOnLAN() []nethelpers.WOLMode {
	return s.WakeOnLANConfig
}

// Validate implements config.Validator interface.
func (s *EthernetConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.MetaName == "" {
		return nil, errors.New("name is required")
	}

	return nil, nil
}
