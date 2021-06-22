// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/talos-systems/crypto/x509"
	"google.golang.org/protobuf/proto"

	resourceapi "github.com/talos-systems/talos/pkg/machinery/api/resource"
)

// APIType is type of API resource.
const APIType = resource.Type("ApiCertificates.secrets.talos.dev")

// APIID is a resource ID of singleton instance.
const APIID = resource.ID("api")

// API contains apid generated secrets.
type API struct {
	md   resource.Metadata
	spec *APICertsSpec
}

// APICertsSpec describes etcd certs secrets.
type APICertsSpec struct {
	CA     *x509.PEMEncodedCertificateAndKey `yaml:"ca"` // only cert is passed, without key
	Client *x509.PEMEncodedCertificateAndKey `yaml:"client"`
	Server *x509.PEMEncodedCertificateAndKey `yaml:"server"`
}

// MarshalProto implements ProtoMarshaler.
func (spec *APICertsSpec) MarshalProto() ([]byte, error) {
	protoSpec := resourceapi.APISpec{
		Ca:         spec.CA.Crt,
		ClientCert: spec.Client.Crt,
		ClientKey:  spec.Client.Key,
		ServerCert: spec.Server.Crt,
		ServerKey:  spec.Server.Key,
	}

	return proto.Marshal(&protoSpec)
}

// NewAPI initializes a Etc resource.
func NewAPI() *API {
	r := &API{
		md:   resource.NewMetadata(NamespaceName, APIType, APIID, resource.VersionUndefined),
		spec: &APICertsSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *API) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *API) Spec() interface{} {
	return r.spec
}

func (r *API) String() string {
	return fmt.Sprintf("secrets.APICertificates(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *API) DeepCopy() resource.Resource {
	specCopy := *r.spec

	return &API{
		md:   r.md,
		spec: &specCopy,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *API) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             APIType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec returns .spec.
func (r *API) TypedSpec() *APICertsSpec {
	return r.spec
}

// UnmarshalProto implements protobuf.ResourceUnmarshaler.
func (r *API) UnmarshalProto(md *resource.Metadata, protoBytes []byte) error {
	r.md = *md

	protoSpec := resourceapi.APISpec{}

	if err := proto.Unmarshal(protoBytes, &protoSpec); err != nil {
		return err
	}

	r.spec = &APICertsSpec{
		CA: &x509.PEMEncodedCertificateAndKey{
			Crt: protoSpec.Ca,
		},
		Client: &x509.PEMEncodedCertificateAndKey{
			Crt: protoSpec.ClientCert,
			Key: protoSpec.ClientKey,
		},
		Server: &x509.PEMEncodedCertificateAndKey{
			Crt: protoSpec.ServerCert,
			Key: protoSpec.ServerKey,
		},
	}

	return nil
}

func init() {
	if err := protobuf.RegisterResource(APIType, &API{}); err != nil {
		panic(err)
	}
}
