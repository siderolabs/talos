// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/resource/protobuf"
	"github.com/cosi-project/runtime/pkg/resource/typed"

	"github.com/siderolabs/talos/pkg/machinery/proto"
)

// WatchdogTimerStatusType is type of WatchdogTimerStatus resource.
const WatchdogTimerStatusType = resource.Type("WatchdogTimerStatuses.runtime.talos.dev")

// WatchdogTimerStatus resource holds status of watchdog timer.
type WatchdogTimerStatus = typed.Resource[WatchdogTimerStatusSpec, WatchdogTimerStatusExtension]

// WatchdogTimerStatusSpec describes configuration of watchdog timer.
//
//gotagsrewrite:gen
type WatchdogTimerStatusSpec struct {
	Device       string        `yaml:"device" protobuf:"1"`
	Timeout      time.Duration `yaml:"timeout" protobuf:"2"`
	FeedInterval time.Duration `yaml:"feedInterval" protobuf:"3"`
}

// NewWatchdogTimerStatus initializes a WatchdogTimerStatus resource.
func NewWatchdogTimerStatus(id string) *WatchdogTimerStatus {
	return typed.NewResource[WatchdogTimerStatusSpec, WatchdogTimerStatusExtension](
		resource.NewMetadata(NamespaceName, WatchdogTimerStatusType, id, resource.VersionUndefined),
		WatchdogTimerStatusSpec{},
	)
}

// WatchdogTimerStatusExtension is auxiliary resource data for WatchdogTimerStatus.
type WatchdogTimerStatusExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (WatchdogTimerStatusExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             WatchdogTimerStatusType,
		Aliases:          []resource.Type{},
		DefaultNamespace: NamespaceName,
		PrintColumns: []meta.PrintColumn{
			{
				Name:     "Device",
				JSONPath: `{.device}`,
			},
			{
				Name:     "Timeout",
				JSONPath: `{.timeout}`,
			},
		},
	}
}

func init() {
	proto.RegisterDefaultTypes()

	err := protobuf.RegisterDynamic[WatchdogTimerStatusSpec](WatchdogTimerStatusType, &WatchdogTimerStatus{})
	if err != nil {
		panic(err)
	}
}
