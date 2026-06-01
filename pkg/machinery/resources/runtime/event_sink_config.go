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

// EventSinkConfigType is type of EventSinkConfig resource.
const EventSinkConfigType = resource.Type("EventSinkConfigs.runtime.talos.dev")

// EventSinkConfig resource holds configuration for Talos event log streaming.
type EventSinkConfig = typed.Resource[EventSinkConfigSpec, EventSinkConfigExtension]

// EventSinkConfigID is a resource ID for EventSinkConfig.
const EventSinkConfigID resource.ID = "event-sink"

// EventSinkConfigSpec describes configuration of Talos event log streaming.
//
//gotagsrewrite:gen
type EventSinkConfigSpec struct {
	Endpoint string `yaml:"endpoint" protobuf:"1"`
}

// NewEventSinkConfig initializes a EventSinkConfig resource.
func NewEventSinkConfig() *EventSinkConfig {
	return typed.NewResource[EventSinkConfigSpec, EventSinkConfigExtension](
		resource.NewMetadata(NamespaceName, EventSinkConfigType, EventSinkConfigID, resource.VersionUndefined),
		EventSinkConfigSpec{},
	)
}

// EventSinkConfigExtension is auxiliary resource data for EventSinkConfig.
type EventSinkConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (EventSinkConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             EventSinkConfigType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[EventSinkConfigSpec](EventSinkConfigType, &EventSinkConfig{})
	if err != nil {
		panic(err)
	}
}
