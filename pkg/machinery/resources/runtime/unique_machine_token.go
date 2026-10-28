// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
)

const (
	// UniqueMachineTokenType is type of [UniqueMachineToken] resource.
	UniqueMachineTokenType = resource.Type("UniqueMachineTokens.runtime.talos.dev")

	// UniqueMachineTokenID is the ID of [UniqueMachineToken] resource.
	UniqueMachineTokenID = resource.ID("unique-machine-token")
)

// UniqueMachineToken resource appears when all meta keys are loaded.
type UniqueMachineToken = typed.Resource[UniqueMachineTokenSpec, UniqueMachineTokenExtension]

// UniqueMachineTokenSpec is the spec for the machine unique token. Token can be empty if machine wasn't assigned any.
//
//gotagsrewrite:gen
//redactgen:gen
type UniqueMachineTokenSpec struct {
	Token string `yaml:"token" protobuf:"1" redact:"replace"`
}

// NewUniqueMachineToken initializes a [UniqueMachineToken] resource.
func NewUniqueMachineToken() *UniqueMachineToken {
	return typed.NewResource[UniqueMachineTokenSpec, UniqueMachineTokenExtension](
		resource.NewMetadata(NamespaceName, UniqueMachineTokenType, UniqueMachineTokenID, resource.VersionUndefined),
		UniqueMachineTokenSpec{},
	)
}

// UniqueMachineTokenExtension is auxiliary resource data for [UniqueMachineToken].
type UniqueMachineTokenExtension struct{}

// ResourceDefinition implements [meta.ResourceDefinitionProvider] interface.
func (UniqueMachineTokenExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             UniqueMachineTokenType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Token",
				JSONPath: `{.token}`,
			},
		},
		// the token authenticates this machine to Omni
		Sensitivity: meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[UniqueMachineTokenSpec](UniqueMachineTokenType, &UniqueMachineToken{})
	if err != nil {
		panic(err)
	}

	// the spec is sensitive, but it is safe to be read in the redacted form
	redactable.Register[UniqueMachineTokenSpec, UniqueMachineTokenExtension]()
}
