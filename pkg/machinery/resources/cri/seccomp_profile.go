// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// SeccompProfileType is type of SeccompProfile resource.
const SeccompProfileType = resource.Type("SeccompProfiles.cri.talos.dev")

// SeccompProfile represents SeccompProfile typed resource.
type SeccompProfile = typed.Resource[SeccompProfileSpec, SeccompProfileExtension]

// SeccompProfileSpec represents the SeccompProfile.
//
//gotagsrewrite:gen
type SeccompProfileSpec struct {
	Name  string         `yaml:"name" protobuf:"1"`
	Value map[string]any `yaml:"value" protobuf:"2"`
}

// NewSeccompProfile creates new SeccompProfile object.
func NewSeccompProfile(id string) *SeccompProfile {
	return typed.NewResource[SeccompProfileSpec, SeccompProfileExtension](
		resource.NewMetadata(NamespaceName, SeccompProfileType, id, resource.VersionUndefined),
		SeccompProfileSpec{},
	)
}

// SeccompProfileExtension is an auxiliary type for SeccompProfile resource.
type SeccompProfileExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (SeccompProfileExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             SeccompProfileType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns:     []meta.PrintColumn{},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[SeccompProfileSpec](SeccompProfileType, &SeccompProfile{})
	if err != nil {
		panic(err)
	}
}
