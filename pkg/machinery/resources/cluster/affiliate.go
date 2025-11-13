// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"net/netip"
	"slices"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/gen/value"

	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/proto"
)

//go:generate go tool github.com/siderolabs/deep-copy -type AffiliateSpec -type ConfigSpec -type IdentitySpec -type MemberSpec -type InfoSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// AffiliateType is type of Affiliate resource.
const AffiliateType = resource.Type("Affiliates.cluster.talos.dev")

// Affiliate resource holds information about cluster affiliate: it is discovered potential cluster member and/or KubeSpan peer.
//
// Controller builds local Affiliate structure for the node itself, other Affiliates are pulled from the registry during the discovery process.
type Affiliate = typed.Resource[AffiliateSpec, AffiliateExtension]

// KubeSpanAffiliateSpec describes additional information specific for the KubeSpan.
//
//gotagsrewrite:gen
type KubeSpanAffiliateSpec struct {
	PublicKey           string           `yaml:"publicKey" protobuf:"1"`
	Address             netip.Addr       `yaml:"address" protobuf:"2"`
	AdditionalAddresses []netip.Prefix   `yaml:"additionalAddresses" protobuf:"3"`
	Endpoints           []netip.AddrPort `yaml:"endpoints" protobuf:"4"`
}

// NewAffiliate initializes the Affiliate resource.
func NewAffiliate(namespace resource.Namespace, id resource.ID) *Affiliate {
	return typed.NewResource[AffiliateSpec, AffiliateExtension](
		resource.NewMetadata(namespace, AffiliateType, id, resource.VersionUndefined),
		AffiliateSpec{},
	)
}

// AffiliateExtension provides auxiliary methods for Affiliate.
type AffiliateExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (r AffiliateExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             AffiliateType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Hostname",
				JSONPath: `{.hostname}`,
			},
			{
				Name:     "Machine Type",
				JSONPath: `{.machineType}`,
			},
			{
				Name:     "Addresses",
				JSONPath: `{.addresses}`,
			},
		},
	}
}

// AffiliateSpec describes Affiliate state.
//
//gotagsrewrite:gen
type AffiliateSpec struct {
	NodeID          string                `yaml:"nodeId" protobuf:"1"`
	Addresses       []netip.Addr          `yaml:"addresses" protobuf:"2"`
	Hostname        string                `yaml:"hostname" protobuf:"3"`
	Nodename        string                `yaml:"nodename,omitempty" protobuf:"4"`
	OperatingSystem string                `yaml:"operatingSystem" protobuf:"5"`
	MachineType     machine.Type          `yaml:"machineType" protobuf:"6"`
	KubeSpan        KubeSpanAffiliateSpec `yaml:"kubespan,omitempty" protobuf:"7"`
	ControlPlane    *ControlPlane         `yaml:"controlPlane,omitempty" protobuf:"8"`
}

// ControlPlane describes ControlPlane data if any.
//
//gotagsrewrite:gen
type ControlPlane struct {
	APIServerPort int `yaml:"port" protobuf:"1"`
}

// Merge two AffiliateSpecs.
//
//nolint:gocyclo
func (spec *AffiliateSpec) Merge(other *AffiliateSpec) {
	for _, addr := range other.Addresses {
		found := slices.Contains(spec.Addresses, addr)

		if !found {
			spec.Addresses = append(spec.Addresses, addr)
		}
	}

	if other.ControlPlane != nil {
		spec.ControlPlane = other.ControlPlane
	}

	if other.Hostname != "" {
		spec.Hostname = other.Hostname
	}

	if other.Nodename != "" {
		spec.Nodename = other.Nodename
	}

	if other.MachineType != machine.TypeUnknown {
		spec.MachineType = other.MachineType
	}

	if other.KubeSpan.PublicKey != "" {
		spec.KubeSpan.PublicKey = other.KubeSpan.PublicKey
	}

	if !value.IsZero(other.KubeSpan.Address) {
		spec.KubeSpan.Address = other.KubeSpan.Address
	}

	for _, addr := range other.KubeSpan.AdditionalAddresses {
		found := slices.Contains(spec.KubeSpan.AdditionalAddresses, addr)

		if !found {
			spec.KubeSpan.AdditionalAddresses = append(spec.KubeSpan.AdditionalAddresses, addr)
		}
	}

	for _, addr := range other.KubeSpan.Endpoints {
		found := slices.Contains(spec.KubeSpan.Endpoints, addr)

		if !found {
			spec.KubeSpan.Endpoints = append(spec.KubeSpan.Endpoints, addr)
		}
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[AffiliateSpec](AffiliateType, &Affiliate{})
	if err != nil {
		panic(err)
	}
}
