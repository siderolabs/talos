// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// KubeSpanKind is a KubeSpan config document kind.
const KubeSpanKind = "KubeSpanConfig"

func init() {
	registry.Register(KubeSpanKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeSpanConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkKubeSpanConfig        = &KubeSpanConfigV1Alpha1{}
	_ config.Validator                    = &KubeSpanConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeSpanConfigV1Alpha1{}
)

// KubeSpanConfigV1Alpha1 is a config document to configure KubeSpan.
//
//	examples:
//	  - value: exampleKubeSpanV1Alpha1()
//	alias: KubeSpanConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeSpanConfig
type KubeSpanConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Enable the KubeSpan feature.
	//     Cluster discovery should be enabled with cluster.discovery.enabled for KubeSpan to be enabled.
	//   schema:
	//     type: boolean
	ConfigEnabled *bool `yaml:"enabled,omitempty"`

	//   description: |
	//     Control whether Kubernetes pod CIDRs are announced over KubeSpan from the node.
	//     If disabled, CNI handles pod-to-pod traffic encapsulation.
	//     If enabled, KubeSpan takes over pod-to-pod traffic directly.
	//   schema:
	//     type: boolean
	ConfigAdvertiseKubernetesNetworks *bool `yaml:"advertiseKubernetesNetworks,omitempty"`

	//   description: |
	//     Skip sending traffic via KubeSpan if the peer connection state is not up.
	//     This provides configurable choice between connectivity and security.
	//   schema:
	//     type: boolean
	ConfigAllowDownPeerBypass *bool `yaml:"allowDownPeerBypass,omitempty"`

	//   description: |
	//     KubeSpan can collect and publish extra endpoints for each member of the cluster
	//     based on Wireguard endpoint information for each peer.
	//     Disabled by default. Do not enable with high peer counts (>50).
	//   schema:
	//     type: boolean
	ConfigHarvestExtraEndpoints *bool `yaml:"harvestExtraEndpoints,omitempty"`

	//   description: |
	//     KubeSpan link MTU size.
	//     Default value is 1420.
	//   schema:
	//     type: integer
	ConfigMTU *uint32 `yaml:"mtu,omitempty"`

	//   description: |
	//     KubeSpan advanced filtering of network addresses.
	//     Settings are optional and apply only to this node.
	ConfigFilters *KubeSpanFiltersConfig `yaml:"filters,omitempty"`
}

// KubeSpanFiltersConfig configures KubeSpan endpoint filters.
type KubeSpanFiltersConfig struct {
	//   description: |
	//     Filter node addresses which will be advertised as KubeSpan endpoints for peer-to-peer Wireguard connections.
	//
	//     By default, all addresses are advertised, and KubeSpan cycles through all endpoints until it finds one that works.
	//
	//     Default value: no filtering.
	//   examples:
	//     - name: Exclude addresses in 192.168.0.0/16 subnet.
	//       value: '[]string{"0.0.0.0/0", "!192.168.0.0/16", "::/0"}'
	//   schema:
	//     type: array
	//     items:
	//       type: string
	ConfigEndpoints []string `yaml:"endpoints,omitempty"`

	//   description: |
	//     Filter networks (e.g., host addresses, pod CIDRs if enabled) which will be advertised over KubeSpan.
	//
	//     By default, all networks are advertised.
	//     Use this filter to exclude some networks from being advertised.
	//
	//     Note: excluded networks will not be reachable over KubeSpan, so make sure
	//     these networks are still reachable via some other route (e.g., direct connection).
	//
	//     Default value: no filtering.
	//   examples:
	//     - name: Exclude private networks from being advertised.
	//       value: '[]Prefix{{netip.MustParsePrefix("192.168.1.0/24")}}'
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+/\d{1,3}$
	ConfigExcludeAdvertisedNetworks []Prefix `yaml:"excludeAdvertisedNetworks,omitempty"`
}

// NewKubeSpanV1Alpha1 creates a new KubeSpanConfig config document.
func NewKubeSpanV1Alpha1() *KubeSpanConfigV1Alpha1 {
	return &KubeSpanConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       KubeSpanKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleKubeSpanV1Alpha1() *KubeSpanConfigV1Alpha1 {
	cfg := NewKubeSpanV1Alpha1()
	cfg.ConfigEnabled = pointer.To(true)
	cfg.ConfigAdvertiseKubernetesNetworks = pointer.To(false)
	cfg.ConfigAllowDownPeerBypass = pointer.To(false)
	cfg.ConfigHarvestExtraEndpoints = pointer.To(false)
	cfg.ConfigMTU = pointer.To(uint32(1420))
	cfg.ConfigFilters = &KubeSpanFiltersConfig{
		ConfigEndpoints:                 []string{"0.0.0.0/0", "::/0"},
		ConfigExcludeAdvertisedNetworks: []Prefix{{netip.MustParsePrefix("192.168.1.0/24")}, {netip.MustParsePrefix("2003::/16")}},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeSpanConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *KubeSpanConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.ConfigMTU != nil && *s.ConfigMTU < constants.KubeSpanLinkMinimumMTU {
		errs = errors.Join(errs, fmt.Errorf("kubespan link MTU must be at least %d", constants.KubeSpanLinkMinimumMTU))
	}

	if s.ConfigFilters != nil {
		for _, cidr := range s.ConfigFilters.ConfigEndpoints {
			cidr = strings.TrimPrefix(cidr, "!")

			if _, err := sideronet.ParseSubnetOrAddress(cidr); err != nil {
				errs = errors.Join(errs, fmt.Errorf("KubeSpan endpoint filter is not valid: %q", cidr))
			}
		}
	}

	return nil, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeSpanConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	legacyKubespan := v1alpha1Cfg.NetworkKubeSpanConfig()

	if legacyKubespan != nil {
		return fmt.Errorf("kubespan is already configured in v1alpha1 machine.network.kubespan")
	}

	return nil
}

// Enabled implements config.NetworkKubeSpanConfig interface.
func (s *KubeSpanConfigV1Alpha1) Enabled() bool {
	return pointer.SafeDeref(s.ConfigEnabled)
}

// AdvertiseKubernetesNetworks implements config.NetworkKubeSpanConfig interface.
func (s *KubeSpanConfigV1Alpha1) AdvertiseKubernetesNetworks() bool {
	return pointer.SafeDeref(s.ConfigAdvertiseKubernetesNetworks)
}

// ForceRouting implements config.NetworkKubeSpanConfig interface.
func (s *KubeSpanConfigV1Alpha1) ForceRouting() bool {
	return !pointer.SafeDeref(s.ConfigAllowDownPeerBypass)
}

// HarvestExtraEndpoints implements config.NetworkKubeSpanConfig interface.
func (s *KubeSpanConfigV1Alpha1) HarvestExtraEndpoints() bool {
	return pointer.SafeDeref(s.ConfigHarvestExtraEndpoints)
}

// MTU implements config.NetworkKubeSpanConfig interface.
func (s *KubeSpanConfigV1Alpha1) MTU() uint32 {
	if s.ConfigMTU != nil {
		return *s.ConfigMTU
	}

	return constants.KubeSpanLinkMTU
}

// Filters implements config.NetworkKubeSpanConfig interface.
func (s *KubeSpanConfigV1Alpha1) Filters() config.NetworkKubeSpanFilters {
	if s.ConfigFilters == nil {
		return nil
	}

	return s.ConfigFilters
}

// Endpoints implements config.NetworkKubeSpanFilters interface.
func (f *KubeSpanFiltersConfig) Endpoints() []string {
	return f.ConfigEndpoints
}

// ExcludeAdvertisedNetworks implements config.NetworkKubeSpanFilters interface.
func (f *KubeSpanFiltersConfig) ExcludeAdvertisedNetworks() []netip.Prefix {
	return xslices.Map(f.ConfigExcludeAdvertisedNetworks, func(p Prefix) netip.Prefix { return p.Prefix })
}
