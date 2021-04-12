// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/talos-systems/crypto/x509"
)

// KubernetesType is type of Kubernetes resource.
const KubernetesType = resource.Type("KubernetesSecrets.secrets.talos.dev")

// KubernetesID is a resource ID of singleton instance.
const KubernetesID = resource.ID("k8s-certs")

// Kubernetes contains K8s generated secrets.
type Kubernetes struct {
	md   resource.Metadata
	spec *KubernetesCertsSpec
}

// KubernetesCertsSpec describes generated Kubernetes certificates.
type KubernetesCertsSpec struct {
	APIServer              *x509.PEMEncodedCertificateAndKey `yaml:"apiServer"`
	APIServerKubeletClient *x509.PEMEncodedCertificateAndKey `yaml:"apiServerKubeletClient"`
	FrontProxy             *x509.PEMEncodedCertificateAndKey `yaml:"frontProxy"`

	AdminKubeconfig string `yaml:"adminKubeconfig"`
}

// NewKubernetes initializes a Kubernetes resource.
func NewKubernetes() *Kubernetes {
	r := &Kubernetes{
		md:   resource.NewMetadata(NamespaceName, KubernetesType, KubernetesID, resource.VersionUndefined),
		spec: &KubernetesCertsSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Kubernetes) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Kubernetes) Spec() interface{} {
	return r.spec
}

func (r *Kubernetes) String() string {
	return fmt.Sprintf("secrets.KuberneteSecrets(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Kubernetes) DeepCopy() resource.Resource {
	specCopy := *r.spec

	return &Kubernetes{
		md:   r.md,
		spec: &specCopy,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Kubernetes) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// Certs returns .spec.
func (r *Kubernetes) Certs() *KubernetesCertsSpec {
	return r.spec
}
