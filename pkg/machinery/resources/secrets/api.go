// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/talos-systems/crypto/x509"

	secretspb "github.com/talos-systems/talos/pkg/machinery/api/resource/secrets"
	"github.com/talos-systems/talos/pkg/machinery/proto"
)

//nolint:lll
//go:generate deep-copy -type APICertsSpec -type CertSANSpec -type EtcdCertsSpec -type EtcdRootSpec -type KubeletSpec -type KubernetesCertsSpec -type KubernetesRootSpec -type OSRootSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// APIType is type of API resource.
const APIType = resource.Type("ApiCertificates.secrets.talos.dev")

// APIID is a resource ID of singleton instance.
const APIID = resource.ID("api")

// API contains apid generated secrets.
type API = typed.Resource[APICertsSpec, APIRD]

// APICertsSpec describes etcd certs secrets.
//gotagsrewrite:gen
type APICertsSpec struct {
	CA     *x509.PEMEncodedCertificateAndKey `yaml:"ca" protobuf:"1"` // only cert is passed, without key
	Client *x509.PEMEncodedCertificateAndKey `yaml:"client" protobuf:"2"`
	Server *x509.PEMEncodedCertificateAndKey `yaml:"server" protobuf:"3"`
}

// NewAPI initializes a Etc resource.
func NewAPI() *API {
	return typed.NewResource[APICertsSpec, APIRD](
		resource.NewMetadata(NamespaceName, APIType, APIID, resource.VersionUndefined),
		APICertsSpec{},
	)
}

// MarshalProto implements ProtoMarshaler.
func (spec APICertsSpec) MarshalProto() ([]byte, error) {
	protoSpec := secretspb.APISpec{
		CaPem: spec.CA.Crt,
		Client: &secretspb.CertAndKeyPEM{
			Cert: spec.Client.Crt,
			Key:  spec.Client.Key,
		},
		Server: &secretspb.CertAndKeyPEM{
			Cert: spec.Server.Crt,
			Key:  spec.Server.Key,
		},
	}

	return proto.Marshal(&protoSpec)
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (spec *APICertsSpec) UnmarshalProto(protoBytes []byte) error {
	protoSpec := secretspb.APISpec{}

	if err := proto.Unmarshal(protoBytes, &protoSpec); err != nil {
		return err
	}

	*spec = APICertsSpec{
		CA: &x509.PEMEncodedCertificateAndKey{
			Crt: protoSpec.CaPem,
		},
		Client: &x509.PEMEncodedCertificateAndKey{
			Crt: protoSpec.Client.Cert,
			Key: protoSpec.Client.Key,
		},
		Server: &x509.PEMEncodedCertificateAndKey{
			Crt: protoSpec.Server.Cert,
			Key: protoSpec.Server.Key,
		},
	}

	return nil
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
	if err := protobuf.RegisterResource(APIType, &API{}); err != nil {
		panic(err)
	}
}
