// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// DeepCopyable requires a spec to have DeepCopy method which will be used during TypedResource copy.
type DeepCopyable[T any] interface {
	DeepCopy() T
}

// ResourceDefinition is a phantom type which acts as info supplier for TypedResource String and ResourceDefinition
// methods. It intantianed only during String and ResourceDefinition calls, so it should never contain any data which
// survies those calls.
type ResourceDefinition[T any] interface {
	ResourceDefinition(md resource.Metadata, spec T) meta.ResourceDefinitionSpec
	String(md resource.Metadata, spec T) string
}

// TypedResource provides a base implementation for resource.Resource.
type TypedResource[T DeepCopyable[T], RD ResourceDefinition[T]] struct {
	md   resource.Metadata
	spec T
}

// Metadata implements resource.Resource.
func (t *TypedResource[T, RD]) Metadata() *resource.Metadata {
	return &t.md
}

// Spec implements resource.Resource.
func (t *TypedResource[T, RD]) Spec() interface{} {
	return t.spec
}

// TypedSpec returns a pointer to spec field.
func (t *TypedResource[T, RD]) TypedSpec() *T {
	return &t.spec
}

// DeepCopy returns a deep copy of TypedResource.
func (t *TypedResource[T, RD]) DeepCopy() resource.Resource {
	return NewTypedResource[T, RD](t.md, t.spec.DeepCopy())
}

// String implements resource.Resource.
func (t *TypedResource[T, RD]) String() string {
	var zero RD

	return zero.String(t.md, t.spec)
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (t *TypedResource[T, RD]) ResourceDefinition() meta.ResourceDefinitionSpec {
	var zero RD

	return zero.ResourceDefinition(t.md, t.spec)
}

// NewTypedResource initializes and returns a new instance of Resource witth typed spec field.
func NewTypedResource[T DeepCopyable[T], RD ResourceDefinition[T]](md resource.Metadata, spec T) *TypedResource[T, RD] {
	result := TypedResource[T, RD]{md: md, spec: spec}
	result.md.BumpVersion()

	return &result
}
