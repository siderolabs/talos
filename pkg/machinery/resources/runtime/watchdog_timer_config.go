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

// WatchdogTimerConfigType is type of WatchdogTimerConfig resource.
const WatchdogTimerConfigType = resource.Type("WatchdogTimerConfigs.runtime.talos.dev")

// WatchdogTimerConfig resource holds configuration for watchdog timer.
type WatchdogTimerConfig = typed.Resource[WatchdogTimerConfigSpec, WatchdogTimerConfigExtension]

// WatchdogTimerConfigID is a resource ID for WatchdogTimerConfig.
const WatchdogTimerConfigID resource.ID = "timer"

// WatchdogTimerConfigSpec describes configuration of watchdog timer.
//
//gotagsrewrite:gen
type WatchdogTimerConfigSpec struct {
	Device  string        `yaml:"device" protobuf:"1"`
	Timeout time.Duration `yaml:"timeout" protobuf:"2"`
}

// NewWatchdogTimerConfig initializes a WatchdogTimerConfig resource.
func NewWatchdogTimerConfig() *WatchdogTimerConfig {
	return typed.NewResource[WatchdogTimerConfigSpec, WatchdogTimerConfigExtension](
		resource.NewMetadata(NamespaceName, WatchdogTimerConfigType, WatchdogTimerConfigID, resource.VersionUndefined),
		WatchdogTimerConfigSpec{},
	)
}

// WatchdogTimerConfigExtension is auxiliary resource data for WatchdogTimerConfig.
type WatchdogTimerConfigExtension struct{}

// ResourceDefinition implements meta.ResourceDefinitionProvider interface.
func (WatchdogTimerConfigExtension) ResourceDefinition() meta.ResourceDefinitionSpec {
	return meta.ResourceDefinitionSpec{
		Type:             WatchdogTimerConfigType,
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

	err := protobuf.RegisterDynamic[WatchdogTimerConfigSpec](WatchdogTimerConfigType, &WatchdogTimerConfig{})
	if err != nil {
		panic(err)
	}
}
