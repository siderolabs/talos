// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets

import (
	"fmt"
	"net/url"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/talos-systems/crypto/x509"
)

// KubeletType is type of Kubelet secret resource.
const KubeletType = resource.Type("KubeletSecrets.secrets.talos.dev")

// KubeletID is the ID of KubeletType resource.
const KubeletID = resource.ID("kubelet")

// Kubelet contains root (not generated) secrets.
type Kubelet struct {
	md   resource.Metadata
	spec KubeletSpec
}

// KubeletSpec describes root Kubernetes secrets.
type KubeletSpec struct {
	Endpoint *url.URL `yaml:"endpoint"`

	CA *x509.PEMEncodedCertificateAndKey `yaml:"ca"`

	BootstrapTokenID     string `yaml:"bootstrapTokenID"`
	BootstrapTokenSecret string `yaml:"bootstrapTokenSecret"`
}

// NewKubelet initializes a Kubelet resource.
func NewKubelet(id resource.ID) *Kubelet {
	r := &Kubelet{
		md:   resource.NewMetadata(NamespaceName, KubeletType, id, resource.VersionUndefined),
		spec: KubeletSpec{},
	}

	r.md.BumpVersion()

	return r
}

// Metadata implements resource.Resource.
func (r *Kubelet) Metadata() *resource.Metadata {
	return &r.md
}

// Spec implements resource.Resource.
func (r *Kubelet) Spec() interface{} {
	return &r.spec
}

func (r *Kubelet) String() string {
	return fmt.Sprintf("secrets.Kubelet(%q)", r.md.ID())
}

// DeepCopy implements resource.Resource.
func (r *Kubelet) DeepCopy() resource.Resource {
	return &Kubelet{
		md:   r.md,
		spec: r.spec,
	}
}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (r *Kubelet) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             KubeletType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		Sensitivity:      meta.Sensitive,
	}
}

// TypedSpec returns .spec.
func (r *Kubelet) TypedSpec() *KubeletSpec {
	return &r.spec
}
