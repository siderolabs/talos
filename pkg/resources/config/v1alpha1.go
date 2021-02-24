// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"

	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/core"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/encoder"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// V1Alpha1Type is type of Service resource.
const V1Alpha1Type = resource.Type("config/v1alpha1")

// V1Alpha1ID is the ID of V1Alpha1 resource (singleton).
const V1Alpha1ID = resource.ID("v1alpha1")

// V1Alpha1 resource holds v1alpha Talos configuration.
type V1Alpha1 struct {
	md   resource.Metadata
	spec *v1alpha1Spec
}

type v1alpha1Spec struct {
	cfg config.Provider
}

func (s *v1alpha1Spec) MarshalYAML() (interface{}, error) {
	return encoder.NewEncoder(s.cfg).Marshal()
}

// NewV1Alpha1 initializes a V1Alpha1 resource.
func NewV1Alpha1(spec config.Provider) *V1Alpha1 {
	r := &V1Alpha1{
		md: resource.NewMetadata(NamespaceName, V1Alpha1Type, V1Alpha1ID, resource.VersionUndefined),
		spec: &v1alpha1Spec{
			cfg: spec,
		},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *V1Alpha1) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *V1Alpha1) Spec() interface{} {
	return r.spec
}

func (r *V1Alpha1) String() string {
	return fmt.Sprintf("config.V1Alpha1(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *V1Alpha1) DeepCopy() resource.Resource {
	b, err := r.spec.cfg.Bytes()
	if err != nil {
		panic(err) // TODO: DeepCopy() should support returning errors? or config should implement DeeCopy without errors?
	}

	c, err := configloader.NewFromBytes(b)
	if err != nil {
		panic(err)
	}

	return &V1Alpha1{
		md: r.md,
		spec: &v1alpha1Spec{
			cfg: c.(*v1alpha1.Config),
		},
	}
}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (r *V1Alpha1) ResourceDefinition() core.ResourceDefinitionSpec {
	return core.ResourceDefinitionSpec{
		Type:             V1Alpha1Type,
		Aliases:          []resource.Type{Type},
		DefaultNamespace: NamespaceName,
	}
}

// Config returns config.Provider.
func (r *V1Alpha1) Config() config.Provider {
	return r.spec.cfg
}
