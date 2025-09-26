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

// OOMActionType is the type of the OOM action record resource.
const OOMActionType = resource.Type("OOMActions.talos.dev")

// OOMAction is the OOM action record resource.
type OOMAction = typed.Resource[OOMActionSpec, OOMActionExtension]

// OOMActionSpec describes the OOM action record resource properties.
//
//gotagsrewrite:gen
type OOMActionSpec struct {
	TriggerContext string   `yaml:"triggerContext,omitempty" protobuf:"1"`
	Score          float64  `yaml:"score,omitempty" protobuf:"2"`
	Processes      []string `yaml:"processes,omitempty" protobuf:"3"`
}

// NewOOMActionSpec initializes an OOM action log entry resource.
func NewOOMActionSpec(namespace resource.Namespace, id resource.ID) *OOMAction {
	return typed.NewResource[OOMActionSpec, OOMActionExtension](
		resource.NewMetadata(namespace, OOMActionType, id, resource.VersionUndefined),
		OOMActionSpec{},
	)
}

// OOMActionExtension provides auxiliary methods for OOMAction.
type OOMActionExtension struct{}

// ResourceDefinition implements [typed.Extension] interface.
func (OOMActionExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             OOMActionType,
		DefaultNamespace: NamespaceName,
		Aliases:          []string{"oomaction", "oomactions"},
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Time",
				JSONPath: `{.time}`,
			},
			{
				Name:     "Score",
				JSONPath: `{.score}`,
			},
		},
		Sensitivity: meta.Sensitive,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[OOMActionSpec](OOMActionType, &OOMAction{})
	if err != nil {
		panic(err)
	}
}
