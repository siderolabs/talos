// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// BootstrapStatusType is type of BootstrapStatus resource.
const BootstrapStatusType = resource.Type("BootstrapStatuses.v1alpha1.talos.dev")

// BootstrapStatusID is a singleton instance ID.
const BootstrapStatusID = resource.ID("control-plane")

// BootstrapStatus describes v1alpha1 (bootkube) bootstrap status.
type BootstrapStatus struct {
	md   resource.Metadata
	spec BootstrapStatusSpec
}

// BootstrapStatusSpec describe service state.
type BootstrapStatusSpec struct {
	SelfHostedControlPlane bool `yaml:"selfHostedControlPlane"`
}

// NewBootstrapStatus initializes a BootstrapStatus resource.
func NewBootstrapStatus() *BootstrapStatus {
	r := &BootstrapStatus{
		md:   resource.NewMetadata(NamespaceName, BootstrapStatusType, BootstrapStatusID, resource.VersionUndefined),
		spec: BootstrapStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *BootstrapStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *BootstrapStatus) Spec() interface{} {
	return r.spec
}

func (r *BootstrapStatus) String() string {
	return fmt.Sprintf("v1alpha1.BootstrapStatus(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *BootstrapStatus) DeepCopy() resource.Resource {
	return &BootstrapStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *BootstrapStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             BootstrapStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Self Hosted",
				JSONPath: "{.selfHostedControlPlane}",
			},
		},
	}
}

// Status returns .spec.
func (r *BootstrapStatus) Status() *BootstrapStatusSpec {
	return &r.spec
}
