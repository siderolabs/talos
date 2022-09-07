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

	"github.com/talos-systems/talos/pkg/machinery/proto"
)

// APIType is type of API resource.
const APIType = resource.Type("ApiCertificates.secrets.talos.dev")

// APIID is a resource ID of singleton instance.
const APIID = resource.ID("api")

// API contains apid generated secrets.
type API = typed.Resource[APICertsSpec, APIRD]

// APICertsSpec describes etcd certs secrets.
//
//gotagsrewrite:gen
type APICertsSpec struct {
	CA     *x509.PEMEncodedCertificateAndKey `yaml:"ca" protobuf:"1"` // only cert is passed, without key
	Client *x509.PEMEncodedCertificateAndKey `yaml:"client" protobuf:"2"`
	Server *x509.PEMEncodedCertificateAndKey `yaml:"server" protobuf:"3"`
}

// NewAPI initializes an API resource.
func NewAPI() *API {
	return typed.NewResource[APICertsSpec, APIRD](
		resource.NewMetadata(NamespaceName, APIType, APIID, resource.VersionUndefined),
		APICertsSpec{},
	)
}

// APIRD provides auxiliary methods for API.
type APIRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (APIRD) ResourceDefinition(resource.Metadata, APICertsSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             APIType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[APICertsSpec](APIType, &API{}); err != nil {
		panic(err)
	}
}
