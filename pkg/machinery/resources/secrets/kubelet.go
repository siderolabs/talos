// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"net/url"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/siderolabs/crypto/x509"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// KubeletType is type of Kubelet secret resource.
const KubeletType = resource.Type("KubeletSecrets.secrets.talos.dev")

// KubeletID is the ID of KubeletType resource.
const KubeletID = resource.ID("kubelet")

// Kubelet contains root (not generated) secrets.
type Kubelet = typed.Resource[KubeletSpec, KubeletExtension]

// KubeletSpec describes root Kubernetes secrets.
//
//gotagsrewrite:gen
type KubeletSpec struct {
	Endpoint *url.URL `yaml:"endpoint" protobuf:"1"`

	AcceptedCAs []*x509.PEMEncodedCertificate `yaml:"acceptedCAs" protobuf:"5"`

	BootstrapTokenID     string `yaml:"bootstrapTokenID" protobuf:"3"`
	BootstrapTokenSecret string `yaml:"bootstrapTokenSecret" protobuf:"4"`
}

// NewKubelet initializes a Kubelet resource.
func NewKubelet(id resource.ID) *Kubelet {
	return typed.NewResource[KubeletSpec, KubeletExtension](
		resource.NewMetadata(NamespaceName, KubeletType, id, resource.VersionUndefined),
		KubeletSpec{},
	)
}

// KubeletExtension provides auxiliary methods for Kubelet.
type KubeletExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KubeletExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[KubeletSpec](KubeletType, &Kubelet{})
	if err != nil {
		panic(err)
	}
}
