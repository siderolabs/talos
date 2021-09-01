// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"inet.af/netaddr"

	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
)

// AffiliateType is type of Affiliate resource.
const AffiliateType = resource.Type("Affiliates.cluster.talos.dev")

// Affiliate resource holds information about cluster affiliate: it is discovered potential cluster member and/or KubeSpan peer.
//
// Controller builds local Affiliate structure for the node itself, other Affiliates are pulled from the registry during the discovery process.
type Affiliate struct {
	md   resource.Metadata
	spec AffiliateSpec
}

// AffiliateSpec describes Affiliate state.
type AffiliateSpec struct {
	Addresses   []netaddr.IP          `yaml:"addresses"`
	Hostname    string                `yaml:"hostname"`
	Nodename    string                `yaml:"nodename,omitempty"`
	MachineType machine.Type          `yaml:"machineType,omitempty"`
	KubeSpan    KubeSpanAffiliateSpec `yaml:"kubespan,omitempty"`
}

// KubeSpanAffiliateSpec describes additional information specific for the KubeSpan.
type KubeSpanAffiliateSpec struct {
	PublicKey           string             `yaml:"publicKey"`
	Address             netaddr.IP         `yaml:"address"`
	AdditionalAddresses []netaddr.IPPrefix `yaml:"additionalAddresses"`
	Endpoints           []netaddr.IPPort   `yaml:"endpoints"`
}

// NewAffiliate initializes a Affiliate resource.
func NewAffiliate(namespace resource.Namespace, id resource.ID) *Affiliate {
	r := &Affiliate{
		md:   resource.NewMetadata(namespace, AffiliateType, id, resource.VersionUndefined),
		spec: AffiliateSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Affiliate) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Affiliate) Spec() interface{} {
	return r.spec
}

func (r *Affiliate) String() string {
	return fmt.Sprintf("cluster.Affiliate(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Affiliate) DeepCopy() resource.Resource {
	return &Affiliate{
		md: r.md,
		spec: AffiliateSpec{
			Addresses:   append([]netaddr.IP(nil), r.spec.Addresses...),
			Hostname:    r.spec.Hostname,
			Nodename:    r.spec.Nodename,
			MachineType: r.spec.MachineType,
			KubeSpan: KubeSpanAffiliateSpec{
				PublicKey:           r.spec.KubeSpan.PublicKey,
				Address:             r.spec.KubeSpan.Address,
				AdditionalAddresses: append([]netaddr.IPPrefix(nil), r.spec.KubeSpan.AdditionalAddresses...),
				Endpoints:           append([]netaddr.IPPort(nil), r.spec.KubeSpan.Endpoints...),
			},
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Affiliate) ResourceDefinition() meta.ResourceDefinitionSpec {
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
				Name:     "Addresses",
				JSONPath: `{.addresses}`,
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *Affiliate) TypedSpec() *AffiliateSpec {
	return &r.spec
}
