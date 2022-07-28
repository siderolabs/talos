// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/typed"
)

//nolint:lll
//go:generate deep-copy -type SeccompProfileSpec -header-file ../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// SeccompProfileType is type of SeccompProfile resource.
const SeccompProfileType = resource.Type("SeccompProfiles.cri.talos.dev")

// SeccompProfile represents SeccompProfile typed resource.
type SeccompProfile = typed.Resource[SeccompProfileSpec, SeccompProfileRD]

// SeccompProfileSpec represents the SeccompProfile.
//gotagsrewrite:gen
type SeccompProfileSpec struct {
	Name  string                 `yaml:"name" protobuf:"1"`
	Value map[string]interface{} `yaml:"value" protobuf:"2"`
}

// NewSeccompProfile creates new SeccompProfile object.
func NewSeccompProfile(id string) *SeccompProfile {
	return typed.NewResource[SeccompProfileSpec, SeccompProfileRD](
		resource.NewMetadata(NamespaceName, SeccompProfileType, id, resource.VersionUndefined),
		SeccompProfileSpec{},
	)
}

// SeccompProfileRD is an auxiliary type for SeccompProfile resource.
type SeccompProfileRD struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (SeccompProfileRD) ResourceDefinition(resource.Metadata, SeccompProfileSpec) meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SeccompProfileType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}
