// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"net"
	"net/url"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/talos-systems/crypto/x509"
)

// KubernetesRootType is type of KubernetesRoot secret resource.
const KubernetesRootType = resource.Type("KubernetesRootSecrets.secrets.talos.dev")

// KubernetesRootID is the ID of KubernetesRootType resource.
const KubernetesRootID = resource.ID("k8s")

// KubernetesRoot contains root (not generated) secrets.
type KubernetesRoot = typed.Resource[KubernetesRootSpec, KubernetesRootRD]

// KubernetesRootSpec describes root Kubernetes secrets.
type KubernetesRootSpec struct {
	Name          string   `yaml:"name"`
	Endpoint      *url.URL `yaml:"endpoint"`
	LocalEndpoint *url.URL `yaml:"local_endpoint"`
	CertSANs      []string `yaml:"certSANs"`
	APIServerIPs  []net.IP `yaml:"apiServerIPs"`
	DNSDomain     string   `yaml:"dnsDomain"`

	CA             *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	ServiceAccount *x509.PEMEncodedKey               `yaml:"serviceAccount"`
	AggregatorCA   *x509.PEMEncodedCertificateAndKey `yaml:"aggregatorCA"`

	AESCBCEncryptionSecret string `yaml:"aesCBCEncryptionSecret"`

	BootstrapTokenID     string `yaml:"bootstrapTokenID"`
	BootstrapTokenSecret string `yaml:"bootstrapTokenSecret"`
}

// NewKubernetesRoot initializes a KubernetesRoot resource.
func NewKubernetesRoot(id resource.ID) *KubernetesRoot {
	return typed.NewResource[KubernetesRootSpec, KubernetesRootRD](
		resource.NewMetadata(NamespaceName, KubernetesRootType, id, resource.VersionUndefined),
		KubernetesRootSpec{},
	)
}

// KubernetesRootRD provides auxiliary methods for KubernetesRoot.
type KubernetesRootRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (KubernetesRootRD) ResourceDefinition(resource.Metadata, KubernetesRootSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubernetesRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}
