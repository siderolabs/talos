// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
)

// NamespaceName contains configuration resources.
const NamespaceName resource.Namespace = v1alpha1.NamespaceName

// KernelParamSpecType is type of KernelParam resource.
const KernelParamSpecType = resource.Type("KernelParamSpecs.runtime.talos.dev")

// KernelParamDefaultSpecType is type of KernelParam resource for default kernel params.
const KernelParamDefaultSpecType = resource.Type("KernelParamDefaultSpecs.runtime.talos.dev")

// KernelParam interface.
type KernelParam interface {
	TypedSpec() *KernelParamSpecSpec
}

// KernelParamSpec resource holds sysctl flags to define.
type KernelParamSpec struct {
	md   resource.Metadata
	spec KernelParamSpecSpec
}

// KernelParamSpecSpec describes status of the defined sysctls.
type KernelParamSpecSpec struct {
	Value        string `yaml:"value"`
	IgnoreErrors bool   `yaml:"ignoreErrors"`
}

// NewKernelParamSpec initializes a KernelParamSpec resource.
func NewKernelParamSpec(namespace resource.Namespace, id resource.ID) *KernelParamSpec {
	r := &KernelParamSpec{
		md:   resource.NewMetadata(namespace, KernelParamSpecType, id, resource.VersionUndefined),
		spec: KernelParamSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *KernelParamSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *KernelParamSpec) Spec() interface{} {
	return r.spec
}

func (r *KernelParamSpec) String() string {
	return fmt.Sprintf("runtime.KernelParamSpec.(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *KernelParamSpec) DeepCopy() resource.Resource {
	return &KernelParamSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *KernelParamSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelParamSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the KernelParamSpecSpec with the proper type.
func (r *KernelParamSpec) TypedSpec() *KernelParamSpecSpec {
	return &r.spec
}

// NewKernelParamDefaultSpec initializes a KernelParamSpec resource.
func NewKernelParamDefaultSpec(namespace resource.Namespace, id resource.ID) *KernelParamDefaultSpec {
	r := &KernelParamDefaultSpec{
		md:   resource.NewMetadata(namespace, KernelParamDefaultSpecType, id, resource.VersionUndefined),
		spec: KernelParamSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// KernelParamDefaultSpec implements meta.ResourceDefinitionProvider interface.
type KernelParamDefaultSpec struct {
	md   resource.Metadata
	spec KernelParamSpecSpec
}

// Metadata implements resource.Resource.
func (r *KernelParamDefaultSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *KernelParamDefaultSpec) Spec() interface{} {
	return r.spec
}

func (r *KernelParamDefaultSpec) String() string {
	return fmt.Sprintf("runtime.KernelParamDefaultSpec.(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *KernelParamDefaultSpec) DeepCopy() resource.Resource {
	return &KernelParamDefaultSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *KernelParamDefaultSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KernelParamDefaultSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the KernelParamSpecSpec with the proper type.
func (r *KernelParamDefaultSpec) TypedSpec() *KernelParamSpecSpec {
	return &r.spec
}
