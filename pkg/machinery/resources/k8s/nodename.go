// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// NodenameType is type of Nodename resource.
const NodenameType = resource.Type("Nodenames.kubernetes.talos.dev")

// NodenameID is a singleton resource ID for Nodename.
const NodenameID = resource.ID("nodename")

// Nodename resource holds Kubernetes nodename.
type Nodename struct {
	md   resource.Metadata
	spec NodenameSpec
}

// NodenameSpec describes Kubernetes nodename.
type NodenameSpec struct {
	Nodename        string `yaml:"nodename"`
	HostnameVersion string `yaml:"hostnameVersion"`
}

// NewNodename initializes a Nodename resource.
func NewNodename(namespace resource.Namespace, id resource.ID) *Nodename {
	r := &Nodename{
		md:   resource.NewMetadata(namespace, NodenameType, id, resource.VersionUndefined),
		spec: NodenameSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Nodename) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Nodename) Spec() interface{} {
	return r.spec
}

func (r *Nodename) String() string {
	return fmt.Sprintf("k8s.Nodename(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Nodename) DeepCopy() resource.Resource {
	return &Nodename{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Nodename) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             NodenameType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Nodename",
				JSONPath: "{.nodename}",
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *Nodename) TypedSpec() *NodenameSpec {
	return &r.spec
}
