// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// HostnameStatusType is type of HostnameStatus resource.
const HostnameStatusType = resource.Type("HostnameStatuses.net.talos.dev")

// HostnameStatus resource holds node hostname.
type HostnameStatus = typed.Resource[HostnameStatusSpec, HostnameStatusRD]

// HostnameStatusSpec describes node nostname.
//
//gotagsrewrite:gen
type HostnameStatusSpec struct {
	Hostname   string `yaml:"hostname" protobuf:"1"`
	Domainname string `yaml:"domainname" protobuf:"2"`
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
	return typed.NewResource[HostnameStatusSpec, HostnameStatusRD](
		resource.NewMetadata(namespace, HostnameStatusType, id, resource.VersionUndefined),
		HostnameStatusSpec{},
	)
}

// HostnameStatusRD provides auxiliary methods for HostnameStatus.
type HostnameStatusRD struct{}

// ResourceDefinition implements typed.ResourceDefinition interface.
func (HostnameStatusRD) ResourceDefinition(resource.Metadata, HostnameStatusSpec) meta.ResourceDefinitionSpec {
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

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[HostnameStatusSpec](HostnameStatusType, &HostnameStatus{})
	if err != nil {
		panic(err)
	}
}
