// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// HostnameSpecType is type of HostnameSpec resource.
const HostnameSpecType = resource.Type("HostnameSpecs.net.talos.dev")

// HostnameSpec resource holds node hostname.
type HostnameSpec = typed.Resource[HostnameSpecSpec, HostnameSpecRD]

// HostnameID is the ID of the singleton instance.
const HostnameID resource.ID = "hostname"

// HostnameSpecSpec describes node nostname.
//
//gotagsrewrite:gen
type HostnameSpecSpec struct {
	Hostname    string      `yaml:"hostname" protobuf:"1"`
	Domainname  string      `yaml:"domainname" protobuf:"2"`
	ConfigLayer ConfigLayer `yaml:"layer" protobuf:"3"`
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
	return typed.NewResource[HostnameSpecSpec, HostnameSpecRD](
		resource.NewMetadata(namespace, HostnameSpecType, id, resource.VersionUndefined),
		HostnameSpecSpec{},
	)
}

// HostnameSpecRD provides auxiliary methods for HostnameSpec.
type HostnameSpecRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (HostnameSpecRD) ResourceDefinition(resource.Metadata, HostnameSpecSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             HostnameSpecType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[HostnameSpecSpec](HostnameSpecType, &HostnameSpec{})
	if err != nil {
		panic(err)
	}
}
