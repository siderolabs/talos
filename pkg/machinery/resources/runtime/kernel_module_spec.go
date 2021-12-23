// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// KernelModuleSpecType is type of KernelModuleSpec resource.
const KernelModuleSpecType = resource.Type("KernelModuleSpecs.runtime.talos.dev")

// KernelModuleSpec resource holds information about Linux kernel module to load.
type KernelModuleSpec struct {
	md   resource.Metadata
	spec KernelModuleSpecSpec
}

// KernelModuleSpecSpec describes Linux kernel module to load.
type KernelModuleSpecSpec struct {
	Name string `yaml:"string"`
	// more options in the future: args, aliases, etc.
}

// NewKernelModuleSpec initializes a KernelModuleSpec resource.
func NewKernelModuleSpec(namespace resource.Namespace, id resource.ID) *KernelModuleSpec {
	r := &KernelModuleSpec{
		md:   resource.NewMetadata(namespace, KernelModuleSpecType, id, resource.VersionUndefined),
		spec: KernelModuleSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *KernelModuleSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *KernelModuleSpec) Spec() interface{} {
	return r.spec
}

func (r *KernelModuleSpec) String() string {
	return fmt.Sprintf("runtime.KernelModuleSpec.(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *KernelModuleSpec) DeepCopy() resource.Resource {
	return &KernelModuleSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *KernelModuleSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelModuleSpecType,
		Aliases:          []resource.Type{"modules"},
		DefaultNamespace: NamespaceName,
	}
}

// TypedSpec allows to access the KernelModuleSpecSpec with the proper type.
func (r *KernelModuleSpec) TypedSpec() *KernelModuleSpecSpec {
	return &r.spec
}
