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

// KubernetesDynamicCertsType is type of KubernetesCerts resource.
const KubernetesDynamicCertsType = resource.Type("KubernetesDynamicCerts.secrets.talos.dev")

// KubernetesDynamicCertsID is a resource ID of singleton instance.
const KubernetesDynamicCertsID = resource.ID("k8s-dynamic-certs")

// KubernetesDynamicCerts contains K8s generated secrets.
//
// KubernetesDynamicCerts resource contains secrets which do not require reload when updated.
type KubernetesDynamicCerts = typed.Resource[KubernetesDynamicCertsSpec, KubernetesDynamicCertsExtension]

// KubernetesDynamicCertsSpec describes generated KubernetesCerts certificates.
//
//gotagsrewrite:gen
type KubernetesDynamicCertsSpec struct {
	APIServer              *x509.PEMEncodedCertificateAndKey `yaml:"apiServer" protobuf:"1"`
	APIServerKubeletClient *x509.PEMEncodedCertificateAndKey `yaml:"apiServerKubeletClient" protobuf:"2"`
	FrontProxy             *x509.PEMEncodedCertificateAndKey `yaml:"frontProxy" protobuf:"3"`
}

// NewKubernetesDynamicCerts initializes a KubernetesCerts resource.
func NewKubernetesDynamicCerts() *KubernetesDynamicCerts {
	return typed.NewResource[KubernetesDynamicCertsSpec, KubernetesDynamicCertsExtension](
		resource.NewMetadata(NamespaceName, KubernetesDynamicCertsType, KubernetesDynamicCertsID, resource.VersionUndefined),
		KubernetesDynamicCertsSpec{},
	)
}

// KubernetesDynamicCertsExtension provides auxiliary methods for KubernetesCerts.
type KubernetesDynamicCertsExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KubernetesDynamicCertsExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesDynamicCertsType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubernetesDynamicCertsSpec](KubernetesDynamicCertsType, &KubernetesDynamicCerts{})
	if err != nil {
		panic(err)
	}
}
