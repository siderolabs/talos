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

// EnvironmentType is type of Environment resource.
const EnvironmentType = resource.Type("Environments.runtime.talos.dev")

// Environment resource holds information about environment variables.
type Environment = typed.Resource[EnvironmentSpec, EnvironmentExtension]

// EnvironmentSpec describes the specification of Environment resource.
//
//gotagsrewrite:gen
type EnvironmentSpec struct {
	Variables []string `yaml:"variables" protobuf:"1"`
}

// Check interfaces.
var _ redactable.Spec = &EnvironmentSpec{}

// HasSecrets implements redactable.Spec interface.
//
// This is hand-written rather than generated, because the variables are a list of "key=value"
// strings: dropping the list, which is all a struct tag can say, would leave nothing to read, while
// keeping the names and taking out the values still tells the reader how the process is configured.
func (spec *EnvironmentSpec) HasSecrets(resource.Metadata) bool {
	return redactable.HasKeyValues(spec.Variables)
}

// RedactSecrets implements redactable.Spec interface.
func (spec *EnvironmentSpec) RedactSecrets(resource.Metadata) {
	spec.Variables = redactable.RedactKeyValues(spec.Variables)
}

// NewEnvironment initializes a Environment resource.
func NewEnvironment(id resource.ID) *Environment {
	return typed.NewResource[EnvironmentSpec, EnvironmentExtension](
		resource.NewMetadata(NamespaceName, EnvironmentType, id, resource.VersionUndefined),
		EnvironmentSpec{},
	)
}

// EnvironmentExtension is auxiliary resource data for Environment.
type EnvironmentExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (EnvironmentExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type: EnvironmentType,
		Aliases: []resource.Type{
			"env",
		},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Variables",
				JSONPath: "{.variables}",
			},
		},
		// the variables of the machine configuration are set on the machined process, and this is
		// its whole environment
		Sensitivity: meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[EnvironmentSpec](EnvironmentType, &Environment{})
	if err != nil {
		panic(err)
	}

	// the spec is sensitive, but it is safe to be read in the redacted form
	redactable.Register[EnvironmentSpec, EnvironmentExtension]()
}
