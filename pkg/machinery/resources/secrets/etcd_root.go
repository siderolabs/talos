// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/talos-systems/crypto/x509"
)

// EtcdRootType is type of EtcdRoot secret resource.
const EtcdRootType = resource.Type("EtcdRootSecrets.secrets.talos.dev")

// EtcdRootID is the IDs of EtcdRoot.
const EtcdRootID = resource.ID("etcd")

// EtcdRoot contains root (not generated) secrets.
type EtcdRoot struct {
	md   resource.Metadata
	spec EtcdRootSpec
}

// EtcdRootSpec describes etcd CA secrets.
type EtcdRootSpec struct {
	EtcdCA *x509.PEMEncodedCertificateAndKey `yaml:"etcdCA"`
}

// NewEtcdRoot initializes a EtcdRoot resource.
func NewEtcdRoot(id resource.ID) *EtcdRoot {
	r := &EtcdRoot{
		md:   resource.NewMetadata(NamespaceName, EtcdRootType, id, resource.VersionUndefined),
		spec: EtcdRootSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *EtcdRoot) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *EtcdRoot) Spec() interface{} {
	return &r.spec
}

// DeepCopy implements resource.Resource.
func (r *EtcdRoot) DeepCopy() resource.Resource {
	return &EtcdRoot{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *EtcdRoot) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec returns .spec.
func (r *EtcdRoot) TypedSpec() *EtcdRootSpec {
	return &r.spec
}
