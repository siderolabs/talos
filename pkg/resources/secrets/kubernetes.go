// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"fmt"

	"github.com/talos-systems/crypto/x509"
	"github.com/talos-systems/os-runtime/pkg/resource"
	"github.com/talos-systems/os-runtime/pkg/resource/core"
)

// KubernetesType is type of Kubernetes resource.
const KubernetesType = resource.Type("secrets/kubernetes")

// KubernetesID is a resource ID of singleton instance.
const KubernetesID = resource.ID("k8s-certs")

// Kubernetes contains K8s generated secrets.
type Kubernetes struct {
	md   resource.Metadata
	spec interface{}
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
	return fmt.Sprintf("secrets.Kubernetes(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Kubernetes) DeepCopy() resource.Resource {
	return &Kubernetes{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements core.ResourceDefinitionProvider interface.
func (r *Kubernetes) ResourceDefinition() core.ResourceDefinitionSpec {
	return core.ResourceDefinitionSpec{
		Type:             KubernetesType,
		Aliases:          []resource.Type{"k8sSecret", "k8sSecrets"},
		DefaultNamespace: NamespaceName,
	}
}

// Certs returns .spec.
func (r *Kubernetes) Certs() *KubernetesCertsSpec {
	return r.spec.(*KubernetesCertsSpec)
}
