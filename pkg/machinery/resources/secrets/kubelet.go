// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"net/url"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/talos-systems/crypto/x509"
)

// KubeletType is type of Kubelet secret resource.
const KubeletType = resource.Type("KubeletSecrets.secrets.talos.dev")

// KubeletID is the ID of KubeletType resource.
const KubeletID = resource.ID("kubelet")

// Kubelet contains root (not generated) secrets.
type Kubelet = typed.Resource[KubeletSpec, KubeletRD]

// KubeletSpec describes root Kubernetes secrets.
type KubeletSpec struct {
	Endpoint *url.URL `yaml:"endpoint"`

	CA *x509.PEMEncodedCertificateAndKey `yaml:"ca"`

	BootstrapTokenID     string `yaml:"bootstrapTokenID"`
	BootstrapTokenSecret string `yaml:"bootstrapTokenSecret"`
}

// NewKubelet initializes a Kubelet resource.
func NewKubelet(id resource.ID) *Kubelet {
	return typed.NewResource[KubeletSpec, KubeletRD](
		resource.NewMetadata(NamespaceName, KubeletType, id, resource.VersionUndefined),
		KubeletSpec{},
	)
}

// DeepCopy implements the DeepCopyable interface.
func (s KubeletSpec) DeepCopy() KubeletSpec {
	return s
}

// KubeletRD provides auxiliary methods for Kubelet.
type KubeletRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KubeletRD) ResourceDefinition(resource.Metadata, KubeletSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}
