// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// TimeServerSpecType is type of TimeServerSpec resource.
const TimeServerSpecType = resource.Type("TimeServerSpecs.net.talos.dev")

// TimeServerSpec resource holds NTP server info.
type TimeServerSpec struct {
	md   resource.Metadata
	spec TimeServerSpecSpec
}

// TimeServerID is the ID of the singleton instance.
const TimeServerID resource.ID = "timeservers"

// TimeServerSpecSpec describes NTP servers.
type TimeServerSpecSpec struct {
	NTPServers  []string    `yaml:"timeServers"`
	ConfigLayer ConfigLayer `yaml:"layer"`
}

// NewTimeServerSpec initializes a TimeServerSpec resource.
func NewTimeServerSpec(namespace resource.Namespace, id resource.ID) *TimeServerSpec {
	r := &TimeServerSpec{
		md:   resource.NewMetadata(namespace, TimeServerSpecType, id, resource.VersionUndefined),
		spec: TimeServerSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *TimeServerSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *TimeServerSpec) Spec() interface{} {
	return r.spec
}

func (r *TimeServerSpec) String() string {
	return fmt.Sprintf("network.TimeServerSpec(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *TimeServerSpec) DeepCopy() resource.Resource {
	return &TimeServerSpec{
		md: r.md,
		spec: TimeServerSpecSpec{
			NTPServers:  append([]string(nil), r.spec.NTPServers...),
			ConfigLayer: r.spec.ConfigLayer,
		},
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *TimeServerSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TimeServerSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *TimeServerSpec) TypedSpec() *TimeServerSpecSpec {
	return &r.spec
}
