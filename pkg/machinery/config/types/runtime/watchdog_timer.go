// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"fmt"
	"net/url"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// WatchdogTimerKind is a watchdog timer config document kind.
const WatchdogTimerKind = "WatchdogTimerConfig"

func init() {
	registry.Register(WatchdogTimerKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &WatchdogTimerV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.RuntimeConfig = &WatchdogTimerV1Alpha1{}
	_ config.Validator     = &WatchdogTimerV1Alpha1{}
)

// Timeout constants.
const (
	MinWatchdogTimeout     = 10 * time.Second
	DefaultWatchdogTimeout = time.Minute
)

// WatchdogTimerV1Alpha1 is a watchdog timer config document.
//
//	examples:
//	  - value: exampleWatchdogTimerV1Alpha1()
//	alias: WatchdogTimerConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/WatchdogTimerConfig
type WatchdogTimerV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	//   description: |
	//     Path to the watchdog device.
	//   examples:
	//     - value: >
	//        "/dev/watchdog0"
	WatchdogDevice string `yaml:"device"`
	//   description: |
	//     Timeout for the watchdog.
	//
	//     If Talos is unresponsive for this duration, the watchdog will reset the system.
	//
	//     Default value is 1 minute, minimum value is 10 seconds.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	WatchdogTimeout time.Duration `yaml:"timeout,omitempty"`
}

// NewWatchdogTimerV1Alpha1 creates a new eventsink config document.
func NewWatchdogTimerV1Alpha1() *WatchdogTimerV1Alpha1 {
	return &WatchdogTimerV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       WatchdogTimerKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleWatchdogTimerV1Alpha1() *WatchdogTimerV1Alpha1 {
	cfg := NewWatchdogTimerV1Alpha1()
	cfg.WatchdogDevice = "/dev/watchdog0"
	cfg.WatchdogTimeout = 2 * time.Minute

	return cfg
}

// Clone implements config.Document interface.
func (s *WatchdogTimerV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Runtime implements config.Config interface.
func (s *WatchdogTimerV1Alpha1) Runtime() config.RuntimeConfig {
	return s
}

// EventsEndpoint implements config.RuntimeConfig interface.
func (s *WatchdogTimerV1Alpha1) EventsEndpoint() *string {
	return nil
}

// KmsgLogURLs implements config.RuntimeConfig interface.
func (s *WatchdogTimerV1Alpha1) KmsgLogURLs() []*url.URL {
	return nil
}

// WatchdogTimer implements config.RuntimeConfig interface.
func (s *WatchdogTimerV1Alpha1) WatchdogTimer() config.WatchdogTimerConfig {
	return s
}

// Device implements config.WatchdogTimerConfig interface.
func (s *WatchdogTimerV1Alpha1) Device() string {
	return s.WatchdogDevice
}

// Timeout implements config.WatchdogTimerConfig interface.
func (s *WatchdogTimerV1Alpha1) Timeout() time.Duration {
	if s.WatchdogTimeout == 0 {
		return DefaultWatchdogTimeout
	}

	return s.WatchdogTimeout
}

// Validate implements config.Validator interface.
func (s *WatchdogTimerV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.WatchdogDevice == "" {
		return nil, fmt.Errorf("watchdog device: empty value")
	}

	if s.WatchdogTimeout != 0 && s.WatchdogTimeout < MinWatchdogTimeout {
		return nil, fmt.Errorf("watchdog timeout: minimum value is %s", MinWatchdogTimeout)
	}

	return nil, nil
}
