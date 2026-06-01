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

// TrustdType is type of Trustd resource.
const TrustdType = resource.Type("TrustdCertificates.secrets.talos.dev")

// TrustdID is a resource ID of singleton instance.
const TrustdID = resource.ID("trustd")

// Trustd contains trustd generated secrets.
type Trustd = typed.Resource[TrustdCertsSpec, TrustdExtension]

// TrustdCertsSpec describes etcd certs secrets.
//
//gotagsrewrite:gen
type TrustdCertsSpec struct {
	AcceptedCAs []*x509.PEMEncodedCertificate     `yaml:"acceptedCAs" protobuf:"3"`
	Server      *x509.PEMEncodedCertificateAndKey `yaml:"server" protobuf:"2"`
}

// NewTrustd initializes a Trustd resource.
func NewTrustd() *Trustd {
	return typed.NewResource[TrustdCertsSpec, TrustdExtension](
		resource.NewMetadata(NamespaceName, TrustdType, TrustdID, resource.VersionUndefined),
		TrustdCertsSpec{},
	)
}

// TrustdExtension provides auxiliary methods for Trustd.
type TrustdExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (TrustdExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             TrustdType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[TrustdCertsSpec](TrustdType, &Trustd{}); err != nil {
		panic(err)
	}
}
