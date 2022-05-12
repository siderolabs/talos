// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/talos-systems/crypto/x509"
)

// EtcdType is type of Etcd resource.
const EtcdType = resource.Type("EtcdSecrets.secrets.talos.dev")

// EtcdID is a resource ID of singleton instance.
const EtcdID = resource.ID("etcd")

// Etcd contains etcd generated secrets.
type Etcd = typed.Resource[EtcdCertsSpec, EtcdRD]

// EtcdCertsSpec describes etcd certs secrets.
type EtcdCertsSpec struct {
	Etcd          *x509.PEMEncodedCertificateAndKey `yaml:"etcd"`
	EtcdPeer      *x509.PEMEncodedCertificateAndKey `yaml:"etcdPeer"`
	EtcdAdmin     *x509.PEMEncodedCertificateAndKey `yaml:"etcdAdmin"`
	EtcdAPIServer *x509.PEMEncodedCertificateAndKey `yaml:"etcdAPIServer"`
}

// NewEtcd initializes a Etc resource.
func NewEtcd() *Etcd {
	return typed.NewResource[EtcdCertsSpec, EtcdRD](
		resource.NewMetadata(NamespaceName, EtcdType, EtcdID, resource.VersionUndefined),
		EtcdCertsSpec{},
	)
}

// EtcdRD provides auxiliary methods for Etcd.
type EtcdRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (EtcdRD) ResourceDefinition(resource.Metadata, EtcdCertsSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}
