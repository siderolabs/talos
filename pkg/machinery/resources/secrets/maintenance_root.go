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

// MaintenanceRootType is type of MaintenanceRoot secret resource.
const MaintenanceRootType = resource.Type("MaintenanceRootSecrets.secrets.talos.dev")

// MaintenanceRootID is the Resource ID for MaintenanceRoot.
const MaintenanceRootID = resource.ID("maintenance")

// MaintenanceRoot contains root secrets for the maintenance service.
type MaintenanceRoot = typed.Resource[MaintenanceRootSpec, MaintenanceRootExtension]

// MaintenanceRootSpec describes maintenance service CA.
//
//gotagsrewrite:gen
type MaintenanceRootSpec struct {
	CA *x509.PEMEncodedCertificateAndKey `yaml:"ca" protobuf:"1"`
}

// NewMaintenanceRoot initializes a MaintenanceRoot resource.
func NewMaintenanceRoot(id resource.ID) *MaintenanceRoot {
	return typed.NewResource[MaintenanceRootSpec, MaintenanceRootExtension](
		resource.NewMetadata(NamespaceName, MaintenanceRootType, id, resource.VersionUndefined),
		MaintenanceRootSpec{},
	)
}

// MaintenanceRootExtension provides auxiliary methods for MaintenanceRoot.
type MaintenanceRootExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (MaintenanceRootExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             MaintenanceRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[MaintenanceRootSpec](MaintenanceRootType, &MaintenanceRoot{}); err != nil {
		panic(err)
	}
}
