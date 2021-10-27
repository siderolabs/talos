// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"fmt"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/talos-systems/crypto/x509"
	"inet.af/netaddr"
)

// OSRootType is type of OSRoot secret resource.
const OSRootType = resource.Type("OSRootSecrets.secrets.talos.dev")

// OSRootID is the Resource ID for OSRoot.
const OSRootID = resource.ID("os")

// OSRoot contains root (not generated) secrets.
type OSRoot struct {
	md   resource.Metadata
	spec OSRootSpec
}

// OSRootSpec describes operating system CA.
type OSRootSpec struct {
	CA              *x509.PEMEncodedCertificateAndKey `yaml:"ca"`
	CertSANIPs      []netaddr.IP                      `yaml:"certSANIPs"`
	CertSANDNSNames []string                          `yaml:"certSANDNSNames"`

	Token string `yaml:"token"`
}

// NewOSRoot initializes a OSRoot resource.
func NewOSRoot(id resource.ID) *OSRoot {
	r := &OSRoot{
		md:   resource.NewMetadata(NamespaceName, OSRootType, id, resource.VersionUndefined),
		spec: OSRootSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *OSRoot) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *OSRoot) Spec() interface{} {
	return &r.spec
}

func (r *OSRoot) String() string {
	return fmt.Sprintf("secrets.OSRoot(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *OSRoot) DeepCopy() resource.Resource {
	return &OSRoot{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *OSRoot) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             OSRootType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec returns .spec.
func (r *OSRoot) TypedSpec() *OSRootSpec {
	return &r.spec
}
