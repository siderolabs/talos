// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"net/netip"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// OSRootType is type of OSRoot secret resource.
const OSRootType = resource.Type("OSRootSecrets.secrets.talos.dev")

// OSRootID is the Resource ID for OSRoot.
const OSRootID = resource.ID("os")

// OSRoot contains root (not generated) secrets.
type OSRoot = typed.Resource[OSRootSpec, OSRootExtension]

// OSRootSpec describes operating system CA.
//
//gotagsrewrite:gen
type OSRootSpec struct {
	IssuingCA       *x509.PEMEncodedCertificateAndKey `yaml:"issuingCA" protobuf:"1"`
	AcceptedCAs     []*x509.PEMEncodedCertificate     `yaml:"acceptedCAs" protobuf:"5"`
	CertSANIPs      []netip.Addr                      `yaml:"certSANIPs" protobuf:"2"`
	CertSANDNSNames []string                          `yaml:"certSANDNSNames" protobuf:"3"`

	Token string `yaml:"token" protobuf:"4"`
}

// NewOSRoot initializes a OSRoot resource.
func NewOSRoot(id resource.ID) *OSRoot {
	return typed.NewResource[OSRootSpec, OSRootExtension](
		resource.NewMetadata(NamespaceName, OSRootType, id, resource.VersionUndefined),
		OSRootSpec{},
	)
}

// OSRootExtension provides auxiliary methods for OSRoot.
type OSRootExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (OSRootExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             OSRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	if err := protobuf.RegisterDynamic[OSRootSpec](OSRootType, &OSRoot{}); err != nil {
		panic(err)
	}
}
