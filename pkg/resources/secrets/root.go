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

// RootType is type of Root secret resource.
const RootType = resource.Type("RootSecrets.secrets.talos.dev")

// IDs of various resources of RootType.
const (
	RootEtcdID       = resource.ID("etcd")
	RootKubernetesID = resource.ID("k8s")
)

// Root contains root (not generated) secrets.
type Root struct {
	md   resource.Metadata
	spec interface{}
}

// RootEtcdSpec describes etcd CA secrets.
type RootEtcdSpec struct {
	EtcdCA *x509.PEMEncodedCertificateAndKey `yaml:"etcdCA"`
}

// RootKubernetesSpec describes root Kubernetes secrets.
type RootKubernetesSpec struct {
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

// NewRoot initializes a Root resource.
func NewRoot(id resource.ID) *Root {
	r := &Root{
		md: resource.NewMetadata(NamespaceName, RootType, id, resource.VersionUndefined),
	}

	switch id {
	case RootEtcdID:
		r.spec = &RootEtcdSpec{}
	case RootKubernetesID:
		r.spec = &RootKubernetesSpec{}
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Root) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Root) Spec() interface{} {
	return r.spec
}

func (r *Root) String() string {
	return fmt.Sprintf("secrets.RootSecret(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Root) DeepCopy() resource.Resource {
	var specCopy interface{}

	switch v := r.spec.(type) {
	case *RootEtcdSpec:
		vv := *v
		specCopy = &vv
	case *RootKubernetesSpec:
		vv := *v
		specCopy = &vv
	default:
		panic("unexpected spec type")
	}

	return &Root{
		md:   r.md,
		spec: specCopy,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Root) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             RootType,
		Aliases:          []resource.Type{"rootSecret", "rootSecrets"},
		DefaultNamespace: NamespaceName,
	}
}

// EtcdSpec returns .spec.
func (r *Root) EtcdSpec() *RootEtcdSpec {
	return r.spec.(*RootEtcdSpec)
}

// KubernetesSpec returns .spec.
func (r *Root) KubernetesSpec() *RootKubernetesSpec {
	return r.spec.(*RootKubernetesSpec)
}
