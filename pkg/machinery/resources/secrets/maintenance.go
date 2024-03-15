// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// MaintenanceServiceCertsType is type of MaintenanceCerts resource.
const MaintenanceServiceCertsType = resource.Type("MaintenanceServiceCertificates.secrets.talos.dev")

// MaintenanceServiceCertsID is a resource ID of singleton instance.
const MaintenanceServiceCertsID = resource.ID("maintenance")

// MaintenanceServiceCerts contains Maintenance Service generated secrets.
type MaintenanceServiceCerts = typed.Resource[MaintenanceServiceCertsSpec, MaintenanceCertsExtension]

// MaintenanceServiceCertsSpec describes maintenance service certs secrets.
//
//gotagsrewrite:gen
type MaintenanceServiceCertsSpec struct {
	CA     *x509.PEMEncodedCertificateAndKey `yaml:"ca" protobuf:"1"` // only cert is passed, without key
	Server *x509.PEMEncodedCertificateAndKey `yaml:"server" protobuf:"2"`
}

// NewMaintenanceServiceCerts initializes an MaintenanceCerts resource.
func NewMaintenanceServiceCerts() *MaintenanceServiceCerts {
	return typed.NewResource[MaintenanceServiceCertsSpec, MaintenanceCertsExtension](
		resource.NewMetadata(NamespaceName, MaintenanceServiceCertsType, MaintenanceServiceCertsID, resource.VersionUndefined),
		MaintenanceServiceCertsSpec{},
	)
}

// MaintenanceCertsExtension provides auxiliary methods for MaintenanceCerts.
type MaintenanceCertsExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MaintenanceCertsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MaintenanceServiceCertsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[MaintenanceServiceCertsSpec](MaintenanceServiceCertsType, &MaintenanceServiceCerts{}); err != nil {
		panic(err)
	}
}
