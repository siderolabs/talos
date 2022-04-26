// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
	"github.com/talos-systems/crypto/x509"
	"inet.af/netaddr"
)

// OSRootType is type of OSRoot secret resource.
const OSRootType = resource.Type("OSRootSecrets.secrets.talos.dev")

// OSRootID is the Resource ID for OSRoot.
const OSRootID = resource.ID("os")

// OSRoot contains root (not generated) secrets.
type OSRoot = typed.Resource[OSRootSpec, OSRootRD]

// OSRootSpec describes operating system CA.
type OSRootSpec struct {
	CA              *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	CertSANIPs      []netaddr.IP                      `yaml:"certSANIPs"`
	CertSANDNSNames []string                          `yaml:"certSANDNSNames"`

	Token string `yaml:"token"`
}

// NewOSRoot initializes a OSRoot resource.
func NewOSRoot(id resource.ID) *OSRoot {
	return typed.NewResource[OSRootSpec, OSRootRD](
		resource.NewMetadata(NamespaceName, OSRootType, id, resource.VersionUndefined),
		OSRootSpec{},
	)
}

// DeepCopy implements the DeepCopyable interface.
func (spec OSRootSpec) DeepCopy() OSRootSpec {
	return spec
}

// OSRootRD provides auxiliary methods for OSRoot.
type OSRootRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (OSRootRD) ResourceDefinition(resource.Metadata, OSRootSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             OSRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}
