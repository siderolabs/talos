// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// WifiKind is a Wifi config document kind.
const WifiKind = "NetworkWifiConfig"

func init() {
	registry.Register(WifiKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &WifiConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkWifiConfig = &WifiConfigV1Alpha1{}
	_ config.NamedDocument     = &WifiConfigV1Alpha1{}
	_ config.Validator         = &WifiConfigV1Alpha1{}
	_ config.SecretDocument    = &WifiConfigV1Alpha1{}
)

// WifiConfigV1Alpha1 is a config document to configure a WiFi (wireless) network interface.
//
//	examples:
//	  - value: exampleWifiConfigV1Alpha1()
//	alias: NetworkWifiConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/NetworkWifiConfig
type WifiConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the wireless link (interface), e.g. `wlan0`.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     ISO/IEC 3166-1 alpha2 country code to set the wireless regulatory domain, e.g. `NL`.
	//
	//     If not set, the regulatory domain is left to the kernel default (world domain).
	WifiCountryCode string `yaml:"countryCode,omitempty"`
	//   description: |
	//     List of wireless networks to connect to (in order of preference).
	//   schemaRequired: true
	WifiNetworks []WifiNetworkConfig `yaml:"networks"`
}

// WifiNetworkConfig describes a single WiFi network.
type WifiNetworkConfig struct {
	//   description: |
	//     SSID (network name) of the wireless network.
	//   schemaRequired: true
	WifiSSID string `yaml:"ssid"`
	//   description: |
	//     Pre-shared key (passphrase) of the wireless network (WPA2-PSK/WPA3-SAE).
	//
	//     If not set, the network is assumed to be open (no authentication).
	WifiPSK string `yaml:"psk,omitempty"`
	//   description: |
	//     Set if the network SSID is hidden (not broadcasted).
	WifiHidden bool `yaml:"hidden,omitempty"`
}

// NewWifiConfigV1Alpha1 creates a new WifiConfig config document.
func NewWifiConfigV1Alpha1(name string) *WifiConfigV1Alpha1 {
	return &WifiConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       WifiKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleWifiConfigV1Alpha1() *WifiConfigV1Alpha1 {
	cfg := NewWifiConfigV1Alpha1("wlan0")
	cfg.WifiCountryCode = "NL"
	cfg.WifiNetworks = []WifiNetworkConfig{
		{
			WifiSSID: "HomeNetwork",
			WifiPSK:  "topsecretpassphrase",
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *WifiConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (s *WifiConfigV1Alpha1) Name() string {
	return s.MetaName
}

// WifiConfig implements NetworkWifiConfig interface.
func (s *WifiConfigV1Alpha1) WifiConfig() {}

// CountryCode implements NetworkWifiConfig interface.
func (s *WifiConfigV1Alpha1) CountryCode() string {
	return s.WifiCountryCode
}

// Networks implements NetworkWifiConfig interface.
func (s *WifiConfigV1Alpha1) Networks() []config.WifiNetwork {
	return xslices.Map(s.WifiNetworks, func(network WifiNetworkConfig) config.WifiNetwork {
		return network
	})
}

// SSID implements config.WifiNetwork interface.
func (n WifiNetworkConfig) SSID() string {
	return n.WifiSSID
}

// PSK implements config.WifiNetwork interface.
func (n WifiNetworkConfig) PSK() string {
	return n.WifiPSK
}

// Hidden implements config.WifiNetwork interface.
func (n WifiNetworkConfig) Hidden() bool {
	return n.WifiHidden
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *WifiConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	if s.WifiCountryCode != "" {
		if len(s.WifiCountryCode) != 2 {
			errs = errors.Join(errs, errors.New("country code must be a two-letter ISO 3166-1 alpha2 code"))
		} else {
			for _, c := range s.WifiCountryCode {
				if c < 'A' || c > 'Z' {
					errs = errors.Join(errs, errors.New("country code must consist of uppercase latin letters"))

					break
				}
			}
		}
	}

	if len(s.WifiNetworks) == 0 {
		errs = errors.Join(errs, errors.New("at least one network must be specified"))
	}

	for i, network := range s.WifiNetworks {
		if l := len(network.WifiSSID); l < 1 || l > 32 {
			errs = errors.Join(errs, fmt.Errorf("SSID must be between 1 and 32 bytes long (network index %d)", i))
		}

		if network.WifiPSK != "" {
			if l := len(network.WifiPSK); l < 8 || l > 63 {
				errs = errors.Join(errs, fmt.Errorf("PSK passphrase must be between 8 and 63 characters long (network index %d)", i))
			}
		}
	}

	return nil, errs
}

// Redact does in-place replacement of secrets with the given string.
func (s *WifiConfigV1Alpha1) Redact(replacement string) {
	for i := range s.WifiNetworks {
		if s.WifiNetworks[i].WifiPSK != "" {
			s.WifiNetworks[i].WifiPSK = replacement
		}
	}
}
