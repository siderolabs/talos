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

// EtcdType is type of Etcd resource.
const EtcdType = resource.Type("EtcdSecrets.secrets.talos.dev")

// EtcdID is a resource ID of singletone instance.
const EtcdID = resource.ID("etcd")

// Etcd contains etcd generated secrets.
type Etcd struct {
	md   resource.Metadata
	spec *EtcdCertsSpec
}

// EtcdCertsSpec describes etcd certs secrets.
type EtcdCertsSpec struct {
	EtcdPeer *x509.PEMEncodedCertificateAndKey `yaml:"etcdPeer"`
}

// NewEtcd initializes a Etc resource.
func NewEtcd() *Etcd {
	r := &Etcd{
		md:   resource.NewMetadata(NamespaceName, EtcdType, EtcdID, resource.VersionUndefined),
		spec: &EtcdCertsSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Etcd) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Etcd) Spec() interface{} {
	return r.spec
}

func (r *Etcd) String() string {
	return fmt.Sprintf("secrets.EtcdSecrets(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Etcd) DeepCopy() resource.Resource {
	specCopy := *r.spec

	return &Etcd{
		md:   r.md,
		spec: &specCopy,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Etcd) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

// Certs returns .spec.
func (r *Etcd) Certs() *EtcdCertsSpec {
	return r.spec
}
