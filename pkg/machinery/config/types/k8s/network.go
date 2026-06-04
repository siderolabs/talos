// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"errors"
	"fmt"
	"net/netip"

	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//docgen:jsonschema

// KubeNetworkConfig defines the KubeNetworkConfig configuration name.
const KubeNetworkConfig = "KubeNetworkConfig"

func init() {
	registry.Register(KubeNetworkConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KubeNetworkConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.K8sNetworkConfig             = &KubeNetworkConfigV1Alpha1{}
	_ config.Validator                    = &KubeNetworkConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &KubeNetworkConfigV1Alpha1{}
)

// KubeNetworkConfigV1Alpha1 configures Kubernetes base network settings.
//
//	examples:
//	  - value: exampleKubeNetworkConfig1V1Alpha1()
//	  - value: exampleKubeNetworkConfig2V1Alpha1()
//	  - value: exampleKubeNetworkConfig3V1Alpha1()
//	alias: KubeNetworkConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KubeNetworkConfig
type KubeNetworkConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The domain used by Kubernetes DNS.
	//     The default is `cluster.local`
	NetworkDNSDomain string `yaml:"dnsDomain"`
	//   description: |
	//     The pod subnet (CIDR), this can be a single value or two values for dual-stack clusters.
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+/\d{1,3}$
	//   schemaRequired: true
	NetworkPodSubnets []meta.Prefix `yaml:"podSubnets" merge:"replace"`
	//   description: |
	//     The service subnet (CIDR), this can be a single value or two values for dual-stack clusters.
	//   schema:
	//     type: array
	//     items:
	//       type: string
	//       pattern: ^[0-9a-f.:]+/\d{1,3}$
	//   schemaRequired: true
	NetworkServiceSubnets []meta.Prefix `yaml:"serviceSubnets" merge:"replace"`
}

// NewKubeNetworkConfigV1Alpha1 creates a new KubeNetworkConfig config document.
func NewKubeNetworkConfigV1Alpha1() *KubeNetworkConfigV1Alpha1 {
	return &KubeNetworkConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       KubeNetworkConfig,
		},
	}
}

func exampleKubeNetworkConfig1V1Alpha1() *KubeNetworkConfigV1Alpha1 {
	cfg := NewKubeNetworkConfigV1Alpha1()
	cfg.NetworkDNSDomain = constants.DefaultDNSDomain
	cfg.NetworkPodSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
	}
	cfg.NetworkServiceSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
	}

	return cfg
}

func exampleKubeNetworkConfig2V1Alpha1() *KubeNetworkConfigV1Alpha1 {
	cfg := NewKubeNetworkConfigV1Alpha1()
	cfg.NetworkDNSDomain = constants.DefaultDNSDomain
	cfg.NetworkPodSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodNet)},
	}
	cfg.NetworkServiceSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceNet)},
	}

	return cfg
}

func exampleKubeNetworkConfig3V1Alpha1() *KubeNetworkConfigV1Alpha1 {
	cfg := NewKubeNetworkConfigV1Alpha1()
	cfg.NetworkDNSDomain = constants.DefaultDNSDomain
	cfg.NetworkPodSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodNet)},
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodNet)},
	}
	cfg.NetworkServiceSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceNet)},
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceNet)},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeNetworkConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *KubeNetworkConfigV1Alpha1) Validate(_ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	validateSubnets := func(subnets []meta.Prefix, minSize int) error {
		if len(subnets) == 0 {
			return errors.New("at least one subnets must be specified")
		}

		if len(subnets) > 2 {
			return errors.New("at most two subnets can be specified for dual-stack clusters")
		}

		var numberIPv4, numberIPv6 int

		for _, subnet := range subnets {
			if !subnet.Prefix.IsValid() {
				return fmt.Errorf("invalid subnet: %s", subnet.Prefix)
			}

			if subnet.Prefix.Masked() != subnet.Prefix {
				return fmt.Errorf("invalid subnet: %s is not a valid CIDR", subnet.Prefix)
			}

			if subnet.Prefix.Addr().Is4() {
				numberIPv4++
			} else {
				numberIPv6++
			}

			size := subnet.Prefix.Addr().BitLen() - subnet.Prefix.Bits()

			// more validations to come later, we need to define node CIDR size setting
			if size < minSize {
				return fmt.Errorf("invalid subnet: %s is too small, it must be at least /%d IPs", subnet.Prefix, minSize)
			}
		}

		if numberIPv4 > 1 || numberIPv6 > 1 {
			return errors.New("at most one IPv4 and one IPv6 subnet can be specified for dual-stack clusters")
		}

		return nil
	}

	if err := validateSubnets(s.NetworkPodSubnets, 4); err != nil {
		errs = errors.Join(errs, errors.New("pod subnets: "+err.Error()))
	}

	if err := validateSubnets(s.NetworkServiceSubnets, 4); err != nil {
		errs = errors.Join(errs, errors.New("service subnets: "+err.Error()))
	}

	if len(s.NetworkPodSubnets) != len(s.NetworkServiceSubnets) {
		errs = errors.Join(errs, errors.New("the number of pod subnets must match the number of service subnets"))
	}

	return warnings, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeNetworkConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ClusterNetwork != nil { //nolint:staticcheck // legacy access
		return errors.New("cluster network config in v1alpha1 config (.machine.cluster.network) can't be used with KubeNetworkConfig document, please remove it to avoid conflicts")
	}

	return nil
}

// PodCIDRs implements the config.ClusterNetwork interface.
func (s *KubeNetworkConfigV1Alpha1) PodCIDRs() []netip.Prefix {
	return xslices.Map(s.NetworkPodSubnets, func(subnet meta.Prefix) netip.Prefix {
		return subnet.Prefix
	})
}

// ServiceCIDRs implements the config.ClusterNetwork interface.
func (s *KubeNetworkConfigV1Alpha1) ServiceCIDRs() []netip.Prefix {
	return xslices.Map(s.NetworkServiceSubnets, func(subnet meta.Prefix) netip.Prefix {
		return subnet.Prefix
	})
}

// DNSDomain implements the config.ClusterNetwork interface.
func (s *KubeNetworkConfigV1Alpha1) DNSDomain() string {
	if s.NetworkDNSDomain == "" {
		return constants.DefaultDNSDomain
	}

	return s.NetworkDNSDomain
}
