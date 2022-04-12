// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
)

// HostnameSpecType is type of HostnameSpec resource.
const HostnameSpecType = resource.Type("HostnameSpecs.net.talos.dev")

// HostnameSpec resource holds node hostname.
type HostnameSpec struct {
	md   resource.Metadata
	spec HostnameSpecSpec
}

// HostnameID is the ID of the singleton instance.
const HostnameID resource.ID = "hostname"

// HostnameSpecSpec describes node nostname.
type HostnameSpecSpec struct {
	Hostname    string      `yaml:"hostname"`
	Domainname  string      `yaml:"domainname"`
	ConfigLayer ConfigLayer `yaml:"layer"`
}

// Validate the hostname.
func (spec *HostnameSpecSpec) Validate() error {
	lenHostname := len(spec.Hostname)

	if lenHostname == 0 || lenHostname > 63 {
		return fmt.Errorf("invalid hostname %q", spec.Hostname)
	}

	if len(spec.FQDN()) > 253 {
		return fmt.Errorf("fqdn is too long: %d", len(spec.FQDN()))
	}

	return nil
}

// FQDN returns the fully-qualified domain name.
func (spec *HostnameSpecSpec) FQDN() string {
	if spec.Domainname == "" {
		return spec.Hostname
	}

	return spec.Hostname + "." + spec.Domainname
}

// ParseFQDN into parts and validate it.
func (spec *HostnameSpecSpec) ParseFQDN(fqdn string) error {
	parts := strings.SplitN(fqdn, ".", 2)

	spec.Hostname = parts[0]

	if len(parts) > 1 {
		spec.Domainname = parts[1]
	}

	return spec.Validate()
}

// NewHostnameSpec initializes a HostnameSpec resource.
func NewHostnameSpec(namespace resource.Namespace, id resource.ID) *HostnameSpec {
	r := &HostnameSpec{
		md:   resource.NewMetadata(namespace, HostnameSpecType, id, resource.VersionUndefined),
		spec: HostnameSpecSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *HostnameSpec) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *HostnameSpec) Spec() interface{} {
	return r.spec
}

// DeepCopy implements resource.Resource.
func (r *HostnameSpec) DeepCopy() resource.Resource {
	return &HostnameSpec{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *HostnameSpec) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             HostnameSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

// TypedSpec allows to access the Spec with the proper type.
func (r *HostnameSpec) TypedSpec() *HostnameSpecSpec {
	return &r.spec
}
