// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"fmt"
	"net"
	"net/url"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/talos-systems/crypto/x509"
)

// KubernetesRootType is type of KubernetesRoot secret resource.
const KubernetesRootType = resource.Type("KubernetesRootSecrets.secrets.talos.dev")

// KubernetesRootID is the ID of KubernetesRootType resource.
const KubernetesRootID = resource.ID("k8s")

// KubernetesRoot contains root (not generated) secrets.
type KubernetesRoot struct {
	md   resource.Metadata
	spec KubernetesRootSpec
}

// KubernetesRootSpec describes root Kubernetes secrets.
type KubernetesRootSpec struct {
	Name         string   `yaml:"name"`
	Endpoint     *url.URL `yaml:"endpoint"`
	CertSANs     []string `yaml:"certSANs"`
	APIServerIPs []net.IP `yaml:"apiServerIPs"`
	DNSDomain    string   `yaml:"dnsDomain"`

	CA             *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	ServiceAccount *x509.PEMEncodedKey               `yaml:"serviceAccount"`
	AggregatorCA   *x509.PEMEncodedCertificateAndKey `yaml:"aggregatorCA"`

	AESCBCEncryptionSecret string `yaml:"aesCBCEncryptionSecret"`

	BootstrapTokenID     string `yaml:"bootstrapTokenID"`
	BootstrapTokenSecret string `yaml:"bootstrapTokenSecret"`
}

// NewKubernetesRoot initializes a KubernetesRoot resource.
func NewKubernetesRoot(id resource.ID) *KubernetesRoot {
	r := &KubernetesRoot{
		md:   resource.NewMetadata(NamespaceName, KubernetesRootType, id, resource.VersionUndefined),
		spec: KubernetesRootSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *KubernetesRoot) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *KubernetesRoot) Spec() interface{} {
	return &r.spec
}

func (r *KubernetesRoot) String() string {
	return fmt.Sprintf("secrets.KubernetesRoot(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *KubernetesRoot) DeepCopy() resource.Resource {
	return &KubernetesRoot{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *KubernetesRoot) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec returns .spec.
func (r *KubernetesRoot) TypedSpec() *KubernetesRootSpec {
	return &r.spec
}
