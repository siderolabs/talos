// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"net/netip"
	"net/url"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// KubernetesRootType is type of KubernetesRoot secret resource.
const KubernetesRootType = resource.Type("KubernetesRootSecrets.secrets.talos.dev")

// KubernetesRootID is the ID of KubernetesRootType resource.
const KubernetesRootID = resource.ID("k8s")

// KubernetesRoot contains root (not generated) secrets.
type KubernetesRoot = typed.Resource[KubernetesRootSpec, KubernetesRootExtension]

// KubernetesRootSpec describes root Kubernetes secrets.
//
//gotagsrewrite:gen
type KubernetesRootSpec struct {
	Name          string       `yaml:"name" protobuf:"1"`
	Endpoint      *url.URL     `yaml:"endpoint" protobuf:"2"`
	LocalEndpoint *url.URL     `yaml:"local_endpoint" protobuf:"3"`
	CertSANs      []string     `yaml:"certSANs" protobuf:"4"`
	APIServerIPs  []netip.Addr `yaml:"apiServerIPs" protobuf:"14"`
	DNSDomain     string       `yaml:"dnsDomain" protobuf:"6"`

	IssuingCA      *x509.PEMEncodedCertificateAndKey `yaml:"issuingCA" protobuf:"7"`
	AcceptedCAs    []*x509.PEMEncodedCertificate     `yaml:"acceptedCAs" protobuf:"15"`
	ServiceAccount *x509.PEMEncodedKey               `yaml:"serviceAccount" protobuf:"8"`
	AggregatorCA   *x509.PEMEncodedCertificateAndKey `yaml:"aggregatorCA" protobuf:"9"`

	AESCBCEncryptionSecret string `yaml:"aesCBCEncryptionSecret" protobuf:"10"`

	BootstrapTokenID     string `yaml:"bootstrapTokenID" protobuf:"11"`
	BootstrapTokenSecret string `yaml:"bootstrapTokenSecret" protobuf:"12"`

	SecretboxEncryptionSecret string `yaml:"secretboxEncryptionSecret" protobuf:"13"`
}

// NewKubernetesRoot initializes a KubernetesRoot resource.
func NewKubernetesRoot(id resource.ID) *KubernetesRoot {
	return typed.NewResource[KubernetesRootSpec, KubernetesRootExtension](
		resource.NewMetadata(NamespaceName, KubernetesRootType, id, resource.VersionUndefined),
		KubernetesRootSpec{},
	)
}

// KubernetesRootExtension provides auxiliary methods for KubernetesRoot.
type KubernetesRootExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KubernetesRootExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubernetesRootSpec](KubernetesRootType, &KubernetesRoot{})
	if err != nil {
		panic(err)
	}
}
