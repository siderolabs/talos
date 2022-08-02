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

// EtcdRootType is type of EtcdRoot secret resource.
const EtcdRootType = resource.Type("EtcdRootSecrets.secrets.talos.dev")

// EtcdRootID is the IDs of EtcdRoot.
const EtcdRootID = resource.ID("etcd")

// EtcdRoot contains root (not generated) secrets.
type EtcdRoot = typed.Resource[EtcdRootSpec, EtcdRootRD]

// EtcdRootSpec describes etcd CA secrets.
//
//gotagsrewrite:gen
type EtcdRootSpec struct {
	EtcdCA *x509.PEMEncodedCertificateAndKey `yaml:"etcdCA" protobuf:"1"`
}

// NewEtcdRoot initializes a EtcdRoot resource.
func NewEtcdRoot(id resource.ID) *EtcdRoot {
	return typed.NewResource[EtcdRootSpec, EtcdRootRD](
		resource.NewMetadata(NamespaceName, EtcdRootType, id, resource.VersionUndefined),
		EtcdRootSpec{},
	)
}

// EtcdRootRD provides auxiliary methods for EtcdRoot.
type EtcdRootRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (EtcdRootRD) ResourceDefinition(resource.Metadata, EtcdRootSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EtcdRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}
