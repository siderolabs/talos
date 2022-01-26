// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"

	"github.com/talos-systems/talos/pkg/machinery/extensions"
)

// ExtensionStatusType is type of Extension resource.
const ExtensionStatusType = resource.Type("ExtensionStatuses.runtime.talos.dev")

// ExtensionStatus resource holds status of installed system extensions.
type ExtensionStatus struct {
	md   resource.Metadata
	spec ExtensionStatusSpec
}

// ExtensionStatusSpec is the spec for system extensions.
type ExtensionStatusSpec = extensions.Layer

// NewExtensionStatus initializes a ExtensionStatus resource.
func NewExtensionStatus(namespace resource.Namespace, id resource.ID) *ExtensionStatus {
	r := &ExtensionStatus{
		md:   resource.NewMetadata(namespace, ExtensionStatusType, id, resource.VersionUndefined),
		spec: ExtensionStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *ExtensionStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *ExtensionStatus) Spec() interface{} {
	return r.spec
}

func (r *ExtensionStatus) String() string {
	return fmt.Sprintf("runtime.ExtensionStatus.(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *ExtensionStatus) DeepCopy() resource.Resource {
	return &ExtensionStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *ExtensionStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             ExtensionStatusType,
		Aliases:          []resource.Type{"extensions"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Name",
				JSONPath: `{.metadata.name}`,
			},
			{
				Name:     "Version",
				JSONPath: `{.metadata.version}`,
			},
		},
	}
}

// TypedSpec allows to access the ExtensionStatusSpec with the proper type.
func (r *ExtensionStatus) TypedSpec() *ExtensionStatusSpec {
	return &r.spec
}
