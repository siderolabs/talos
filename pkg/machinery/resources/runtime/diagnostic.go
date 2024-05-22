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
)

// DiagnosticType is type of Diagnostic resource.
const DiagnosticType = resource.Type("Diagnostics.runtime.talos.dev")

// Diagnostic resource contains warnings produced by Talos Diagnostics.
type Diagnostic = typed.Resource[DiagnosticSpec, DiagnosticExtension]

// DiagnosticSpec is the spec for devices status.
//
//gotagsrewrite:gen
type DiagnosticSpec struct {
	// Short message describing the problem.
	Message string `yaml:"message" protobuf:"1"`
	// Details about the problem.
	Details []string `yaml:"details" protobuf:"2"`
}

// DocumentationURL returns the URL to the documentation for the warning.
func (spec *DiagnosticSpec) DocumentationURL(id string) string {
	return "https://talos.dev/diagnostic/" + id
}

// NewDiagnstic initializes a Diagnostic resource.
func NewDiagnstic(namespace resource.Namespace, id resource.ID) *Diagnostic {
	return typed.NewResource[DiagnosticSpec, DiagnosticExtension](
		resource.NewMetadata(namespace, DiagnosticType, id, resource.VersionUndefined),
		DiagnosticSpec{},
	)
}

// DiagnosticExtension is auxiliary resource data for Diagnostic.
type DiagnosticExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (DiagnosticExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             DiagnosticType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Message",
				JSONPath: `{.message}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[DiagnosticSpec](DiagnosticType, &Diagnostic{})
	if err != nil {
		panic(err)
	}
}
