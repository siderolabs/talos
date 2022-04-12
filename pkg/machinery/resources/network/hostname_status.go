// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// HostnameStatusType is type of HostnameStatus resource.
const HostnameStatusType = resource.Type("HostnameStatuses.net.talos.dev")

// HostnameStatus resource holds node hostname.
type HostnameStatus struct {
	md   resource.Metadata
	spec HostnameStatusSpec
}

// HostnameStatusSpec describes node nostname.
type HostnameStatusSpec struct {
	Hostname   string `yaml:"hostname"`
	Domainname string `yaml:"domainname"`
}

// FQDN returns the fully-qualified domain name.
func (spec *HostnameStatusSpec) FQDN() string {
	if spec.Domainname == "" {
		return spec.Hostname
	}

	return spec.Hostname + "." + spec.Domainname
}

// DNSNames returns DNS names to be added to the certificate based on the hostname and fqdn.
func (spec *HostnameStatusSpec) DNSNames() []string {
	result := []string{spec.Hostname}

	if spec.Domainname != "" {
		result = append(result, spec.FQDN())
	}

	return result
}

// NewHostnameStatus initializes a HostnameStatus resource.
func NewHostnameStatus(namespace resource.Namespace, id resource.ID) *HostnameStatus {
	r := &HostnameStatus{
		md:   resource.NewMetadata(namespace, HostnameStatusType, id, resource.VersionUndefined),
		spec: HostnameStatusSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *HostnameStatus) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *HostnameStatus) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *HostnameStatus) DeepCopy() resource.Resource {
	return &HostnameStatus{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *HostnameStatus) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             HostnameStatusType,
		Aliases:          []resource.Type{"hostname"},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Hostname",
				JSONPath: "{.hostname}",
			},
			{
				Name:     "Domainname",
				JSONPath: "{.domainname}",
			},
		},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *HostnameStatus) TypedSpec() *HostnameStatusSpec {
	return &r.spec
}
