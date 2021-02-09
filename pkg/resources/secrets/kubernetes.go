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

// KubernetesID is ID of the singleton instance.
const KubernetesID = resource.ID("kubernetes")

// Kubernetes contains K8s secrets.
type Kubernetes struct {
	md   resource.Metadata
	spec KubernetesSpec
}

// KubernetesSpec describes Kubernetes resources.
type KubernetesSpec struct {
	EtcdCA   *x509.PEMEncodedCertificateAndKey `yaml:"etcdCA"`
	EtcdPeer *x509.PEMEncodedCertificateAndKey `yaml:"etcdPeer"`

	CA                     *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	APIServer              *x509.PEMEncodedCertificateAndKey `yaml:"apiServer"`
	APIServerKubeletClient *x509.PEMEncodedCertificateAndKey `yaml:"apiServerKubeletClient"`
	ServiceAccount         *x509.PEMEncodedKey               `yaml:"serviceAccount"`
	AggregatorCA           *x509.PEMEncodedCertificateAndKey `yaml:"aggregatorCA"`
	FrontProxy             *x509.PEMEncodedCertificateAndKey `yaml:"frontProxy"`

	AESCBCEncryptionSecret string `yaml:"aesCBCEncryptionSecret"`

	AdminKubeconfig string `yaml:"adminKubeconfig"`

	BootstrapTokenID     string `yaml:"bootstrapTokenID"`
	BootstrapTokenSecret string `yaml:"bootstrapTokenSecret"`
}

// NewKubernetes initializes a Kubernetes resource.
func NewKubernetes() *Kubernetes {
	r := &Kubernetes{
		md:   resource.NewMetadata(NamespaceName, KubernetesType, KubernetesID, resource.VersionUndefined),
		spec: KubernetesSpec{},
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
		Aliases:          []resource.Type{"secrets", "secret"},
		DefaultNamespace: NamespaceName,
	}
}

// Secrets returns .spec.
func (r *Kubernetes) Secrets() *KubernetesSpec {
	return &r.spec
}
