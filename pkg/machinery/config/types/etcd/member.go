// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"errors"
	"fmt"
	"strings"

	sideronet "github.com/siderolabs/net"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

//docgen:jsonschema

// EtcdMemberConfig defines the EtcdMemberConfig configuration name.
const EtcdMemberConfig = "EtcdMemberConfig"

func init() {
	registry.Register(EtcdMemberConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &EtcdMemberConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.EtcdMemberConfig             = &EtcdMemberConfigV1Alpha1{}
	_ config.Validator                    = &EtcdMemberConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &EtcdMemberConfigV1Alpha1{}
	_ container.ControlplaneOnlyConfig    = &EtcdMemberConfigV1Alpha1{}
)

// EtcdMemberConfigV1Alpha1 configures the etcd member running on the machine.
//
//	description: |
//	  The presence of this document enables the etcd member on the machine.
//	  This document requires the `EtcdCAConfig` document to be present as well.
//
//	  This document should only be present on the controlplane machines.
//	examples:
//	  - value: exampleEtcdMemberConfigV1Alpha1()
//	alias: EtcdMemberConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/EtcdMemberConfig
type EtcdMemberConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The container image used to run etcd.
	//
	//     If not set, the default etcd image for the Talos version is used.
	//   examples:
	//     - value: defaultEtcdImage()
	MemberImage string `yaml:"image,omitempty"`
	//   description: |
	//     Extra arguments to supply to etcd.
	//
	//     Note that the following args are not allowed:
	//
	//     - `name`
	//     - `data-dir`
	//     - `initial-cluster-state`
	//     - `listen-peer-urls`
	//     - `listen-client-urls`
	//     - `cert-file`
	//     - `key-file`
	//     - `trusted-ca-file`
	//     - `peer-client-cert-auth`
	//     - `peer-cert-file`
	//     - `peer-trusted-ca-file`
	//     - `peer-key-file`
	//   schema:
	//     type: object
	//     additionalProperties:
	//       oneOf:
	//         - type: string
	//         - type: array
	//           items:
	//             type: string
	MemberExtraArgs meta.Args `yaml:"extraArgs,omitempty"`
	//   description: |
	//     The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.
	//
	//     IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.
	//     Negative subnet matches should be specified last to filter out IPs picked by positive matches.
	//     If not specified, advertised IP is selected as the first routable address of the node.
	//   examples:
	//     - value: >
	//         []string{"10.0.0.0/8"}
	MemberAdvertisedSubnets []string `yaml:"advertisedSubnets,omitempty"`
	//   description: |
	//     The `listenSubnets` field configures the networks for the etcd to listen for peer and client connections.
	//
	//     If `listenSubnets` is not set, but `advertisedSubnets` is set, `listenSubnets` defaults to
	//     `advertisedSubnets`.
	//
	//     If neither `advertisedSubnets` nor `listenSubnets` is set, `listenSubnets` defaults to listen on all addresses.
	//
	//     IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.
	//     Negative subnet matches should be specified last to filter out IPs picked by positive matches.
	MemberListenSubnets []string `yaml:"listenSubnets,omitempty"`
	//   description: |
	//     Leave the promotion of a joining member to the operator.
	//
	//     A member joining an existing etcd cluster is added as a non-voting learner, and by default
	//     Talos promotes it to a voting member once it has caught up with the leader.
	//     With `manualLearnerPromotion` enabled the member stays a learner until it is promoted
	//     through the etcd API, which keeps a non-voting copy of the data on the machine
	//     without changing the quorum size.
	//
	//     The setting is read when the etcd member joins the cluster.
	MemberManualLearnerPromotion bool `yaml:"manualLearnerPromotion,omitempty"`
}

// NewEtcdMemberConfigV1Alpha1 creates a new EtcdMemberConfig config document.
func NewEtcdMemberConfigV1Alpha1() *EtcdMemberConfigV1Alpha1 {
	return &EtcdMemberConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       EtcdMemberConfig,
		},
	}
}

func defaultEtcdImage() string {
	return constants.EtcdImage + ":" + constants.DefaultEtcdVersion
}

func exampleEtcdMemberConfigV1Alpha1() *EtcdMemberConfigV1Alpha1 {
	cfg := NewEtcdMemberConfigV1Alpha1()
	cfg.MemberImage = defaultEtcdImage()
	cfg.MemberExtraArgs = meta.Args{
		"election-timeout": meta.NewArgValue("5000", nil),
	}
	cfg.MemberAdvertisedSubnets = []string{"10.0.0.0/8"}

	return cfg
}

// Clone implements config.Document interface.
func (s *EtcdMemberConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *EtcdMemberConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	for _, cidr := range s.MemberAdvertisedSubnets {
		if _, err := sideronet.ParseSubnetOrAddress(strings.TrimPrefix(cidr, "!")); err != nil {
			errs = errors.Join(errs, fmt.Errorf("etcd advertised subnet is not valid: %q", cidr))
		}
	}

	for _, cidr := range s.MemberListenSubnets {
		if _, err := sideronet.ParseSubnetOrAddress(strings.TrimPrefix(cidr, "!")); err != nil {
			errs = errors.Join(errs, fmt.Errorf("etcd listen subnet is not valid: %q", cidr))
		}
	}

	return nil, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *EtcdMemberConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	return v1alpha1ConflictValidate(v1alpha1Cfg)
}

// ControlplaneOnlyDocument implements container.ControlplaneOnlyConfig interface.
func (s *EtcdMemberConfigV1Alpha1) ControlplaneOnlyDocument() {}

// EtcdMemberConfigSignal implements config.EtcdMemberConfig interface.
func (s *EtcdMemberConfigV1Alpha1) EtcdMemberConfigSignal() {}

// Image implements config.EtcdMemberConfig interface.
func (s *EtcdMemberConfigV1Alpha1) Image() string {
	if s.MemberImage == "" {
		return defaultEtcdImage()
	}

	return s.MemberImage
}

// ExtraArgs implements config.EtcdMemberConfig interface.
func (s *EtcdMemberConfigV1Alpha1) ExtraArgs() map[string][]string {
	return s.MemberExtraArgs.ToMap()
}

// AdvertisedSubnets implements config.EtcdMemberConfig interface.
func (s *EtcdMemberConfigV1Alpha1) AdvertisedSubnets() []string {
	return s.MemberAdvertisedSubnets
}

// ListenSubnets implements config.EtcdMemberConfig interface.
func (s *EtcdMemberConfigV1Alpha1) ListenSubnets() []string {
	if len(s.MemberListenSubnets) > 0 {
		return s.MemberListenSubnets
	}

	// if advertised subnets are set, use them, otherwise listen on all addresses
	return s.MemberAdvertisedSubnets
}

// ManualLearnerPromotion implements config.EtcdMemberConfig interface.
func (s *EtcdMemberConfigV1Alpha1) ManualLearnerPromotion() bool {
	return s.MemberManualLearnerPromotion
}
