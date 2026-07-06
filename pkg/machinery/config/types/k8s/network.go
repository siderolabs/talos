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

// minSubnetHostBits defines the minimum number of bits identifying hosts in a subnet.
// For example:
// - For a IPv4 subnet prefix /28, the host identifier bit length is 32 - 28 = 4 bits.
// - For a IPv6 subnet prefix /124, the host identifier bit length is 128 - 124 = 4 bits.
const minSubnetHostBits = 4

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
	//   description: |
	//     The IPv4 per-node pod CIDR mask size, i.e. the size of the pod CIDR allocated to each node, from the IPv4 pod subnet.
	//     The default is `24`.
	//     Must be between 1 and 32.
	//   schema:
	//     type: integer
	//     minimum: 1
	//     maximum: 32
	//   schemaRequired: false
	NetworkNodeCIDRMaskSizeIPv4 int `yaml:"nodeCIDRMaskSizeIPv4,omitempty"`
	//   description: |
	//     The IPv6 per-node pod CIDR mask size, i.e. the size of the pod CIDR allocated to each node, from the IPv6 pod subnet.
	//     The default is `64`.
	//     Must be between 1 and 128.
	//   schema:
	//     type: integer
	//     minimum: 1
	//     maximum: 128
	//   schemaRequired: false
	NetworkNodeCIDRMaskSizeIPv6 int `yaml:"nodeCIDRMaskSizeIPv6,omitempty"`
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
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
	}
	cfg.NetworkServiceSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
	}

	return cfg
}

func exampleKubeNetworkConfig2V1Alpha1() *KubeNetworkConfigV1Alpha1 {
	cfg := NewKubeNetworkConfigV1Alpha1()
	cfg.NetworkDNSDomain = constants.DefaultDNSDomain
	cfg.NetworkPodSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
	}
	cfg.NetworkServiceSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
	}

	return cfg
}

func exampleKubeNetworkConfig3V1Alpha1() *KubeNetworkConfigV1Alpha1 {
	cfg := NewKubeNetworkConfigV1Alpha1()
	cfg.NetworkDNSDomain = constants.DefaultDNSDomain
	cfg.NetworkPodSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4PodCIDR)},
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv6PodCIDR)},
	}
	cfg.NetworkServiceSubnets = []meta.Prefix{
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv4ServiceCIDR)},
		{Prefix: netip.MustParsePrefix(constants.DefaultIPv6ServiceCIDR)},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *KubeNetworkConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *KubeNetworkConfigV1Alpha1) Validate(_ validation.RuntimeMode, _ ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	nodeMaskIPv4Valid := s.isValidNodeCIDRMaskSizeIPv4()
	nodeMaskIPv6Valid := s.isValidNodeCIDRMaskSizeIPv6()

	if !nodeMaskIPv4Valid {
		errs = errors.Join(errs, fmt.Errorf("nodeCIDRMaskSizeIPv4 must be between 1 and 32"))
	}

	if !nodeMaskIPv6Valid {
		errs = errors.Join(errs, fmt.Errorf("nodeCIDRMaskSizeIPv6 must be between 1 and 128"))
	}

	if err := validateSubnets(s.NetworkPodSubnets, s.validatePodSubnetSize(nodeMaskIPv4Valid, nodeMaskIPv6Valid)); err != nil {
		errs = errors.Join(errs, errors.New("pod subnets: "+err.Error()))
	}

	if err := validateSubnets(s.NetworkServiceSubnets, s.validateServiceSubnetSize); err != nil {
		errs = errors.Join(errs, errors.New("service subnets: "+err.Error()))
	}

	if len(s.NetworkPodSubnets) != len(s.NetworkServiceSubnets) {
		errs = errors.Join(errs, errors.New("the number of pod subnets must match the number of service subnets"))
	}

	return warnings, errs
}

func (s *KubeNetworkConfigV1Alpha1) isValidNodeCIDRMaskSizeIPv4() bool {
	return s.NetworkNodeCIDRMaskSizeIPv4 == 0 ||
		(s.NetworkNodeCIDRMaskSizeIPv4 >= 1 && s.NetworkNodeCIDRMaskSizeIPv4 <= 32)
}

func (s *KubeNetworkConfigV1Alpha1) isValidNodeCIDRMaskSizeIPv6() bool {
	return s.NetworkNodeCIDRMaskSizeIPv6 == 0 ||
		(s.NetworkNodeCIDRMaskSizeIPv6 >= 1 && s.NetworkNodeCIDRMaskSizeIPv6 <= 128)
}

func (s *KubeNetworkConfigV1Alpha1) validatePodSubnetSize(nodeMaskIPv4Valid, nodeMaskIPv6Valid bool) func(netip.Prefix) error {
	return func(podSubnet netip.Prefix) error {
		perNodePodMaskSize := s.NodeCIDRMaskSizeIPv4()
		isPerNodePodCIDRValid := nodeMaskIPv4Valid

		if podSubnet.Addr().Is6() {
			perNodePodMaskSize = s.NodeCIDRMaskSizeIPv6()
			isPerNodePodCIDRValid = nodeMaskIPv6Valid
		}

		if !isPerNodePodCIDRValid {
			return nil
		}

		// Note that the mask size is inversely proportional to the number of hosts in the subnet.
		if podSubnet.Bits() > perNodePodMaskSize {
			return fmt.Errorf(
				"invalid subnet: %s is smaller than the per-node pod CIDR mask size /%d",
				podSubnet,
				perNodePodMaskSize,
			)
		}

		if perNodePodMaskSize-podSubnet.Bits() > constants.PodSubnetNodeMaskMaxDiff {
			return fmt.Errorf(
				"invalid subnet: %s is too large for the per-node pod CIDR mask size /%d, the difference must be at most %d bits",
				podSubnet,
				perNodePodMaskSize,
				constants.PodSubnetNodeMaskMaxDiff,
			)
		}

		return nil
	}
}

func (s *KubeNetworkConfigV1Alpha1) validateServiceSubnetSize(serviceSubnet netip.Prefix) error {
	if hostBits := serviceSubnet.Addr().BitLen() - serviceSubnet.Bits(); hostBits > constants.MaxHostBitsForServiceSubnet {
		return fmt.Errorf(
			"invalid subnet: %s is too large, it must be at least /%d (at most %d host identifier bits)",
			serviceSubnet,
			serviceSubnet.Addr().BitLen()-constants.MaxHostBitsForServiceSubnet,
			constants.MaxHostBitsForServiceSubnet,
		)
	}

	return nil
}

// validateSubnets validates the given subnets and checks their sizes using the provided checkSize function.
func validateSubnets(subnets []meta.Prefix, checkSize func(netip.Prefix) error) error {
	if len(subnets) == 0 {
		return errors.New("at least one subnet must be specified")
	}

	if len(subnets) > 2 {
		return errors.New("at most two subnets can be specified for dual-stack clusters")
	}

	var countIPv4, countIPv6 int

	for _, subnet := range subnets {
		isIPv4, err := validateSubnet(subnet, checkSize)
		if err != nil {
			return err
		}

		if isIPv4 {
			countIPv4++
		} else {
			countIPv6++
		}
	}

	if countIPv4 > 1 || countIPv6 > 1 {
		return errors.New("at most one IPv4 and one IPv6 subnet can be specified for dual-stack clusters")
	}

	return nil
}

// validateSubnet validates a single subnet and reports whether it is an IPv4 subnet.
func validateSubnet(subnet meta.Prefix, checkSize func(netip.Prefix) error) (bool, error) {
	if !subnet.Prefix.IsValid() {
		return false, fmt.Errorf("invalid subnet: %s", subnet.Prefix)
	}

	if subnet.Prefix.Masked() != subnet.Prefix {
		return false, fmt.Errorf("invalid subnet: %s is not a valid CIDR", subnet.Prefix)
	}

	hostBits := subnet.Prefix.Addr().BitLen() - subnet.Prefix.Bits()

	if hostBits < minSubnetHostBits {
		return false, fmt.Errorf(
			"invalid subnet: /%d is too small, it must be larger than /%d",
			subnet.Prefix.Bits(),
			subnet.Prefix.Bits()-minSubnetHostBits,
		)
	}

	if err := checkSize(subnet.Prefix); err != nil {
		return false, err
	}

	return subnet.Prefix.Addr().Is4(), nil
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *KubeNetworkConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	if v1alpha1Cfg.ClusterConfig != nil && v1alpha1Cfg.ClusterConfig.ClusterNetwork != nil { //nolint:staticcheck // legacy access
		return errors.New("cluster network config is already set in the v1alpha1 config (.machine.cluster.network). Please remove it and use only the new KubeNetworkConfig document to avoid conflicts")
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

// NodeCIDRMaskSizeIPv4 returns the IPv4 per-node pod CIDR mask size, falling back to the default.
func (s *KubeNetworkConfigV1Alpha1) NodeCIDRMaskSizeIPv4() int {
	if s.NetworkNodeCIDRMaskSizeIPv4 == 0 {
		return constants.DefaultNodeCIDRMaskSizeIPv4
	}

	return s.NetworkNodeCIDRMaskSizeIPv4
}

// NodeCIDRMaskSizeIPv6 returns the IPv6 per-node pod CIDR mask size, falling back to the default.
func (s *KubeNetworkConfigV1Alpha1) NodeCIDRMaskSizeIPv6() int {
	if s.NetworkNodeCIDRMaskSizeIPv6 == 0 {
		return constants.DefaultNodeCIDRMaskSizeIPv6
	}

	return s.NetworkNodeCIDRMaskSizeIPv6
}
