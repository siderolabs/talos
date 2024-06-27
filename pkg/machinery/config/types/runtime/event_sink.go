// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"fmt"
	"net"
	"net/url"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// EventSinkKind is a event sink config document kind.
const EventSinkKind = "EventSinkConfig"

func init() {
	registry.Register(EventSinkKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &EventSinkV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.RuntimeConfig = &EventSinkV1Alpha1{}
	_ config.Validator     = &EventSinkV1Alpha1{}
)

// EventSinkV1Alpha1 is a event sink config document.
//
// docgen:configuration
type EventSinkV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	// The endpoint for the event sink as 'host:port'.
	Endpoint string `yaml:"endpoint" docgen:"{'in': '1.7'}"`
}

// NewEventSinkV1Alpha1 creates a new eventsink config document.
func NewEventSinkV1Alpha1() *EventSinkV1Alpha1 {
	return &EventSinkV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       EventSinkKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleEventSinkV1Alpha1() *EventSinkV1Alpha1 {
	cfg := NewEventSinkV1Alpha1()
	cfg.Endpoint = "192.168.10.3:3247"

	return cfg
}

// Clone implements config.Document interface.
func (s *EventSinkV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Runtime implements config.Config interface.
func (s *EventSinkV1Alpha1) Runtime() config.RuntimeConfig {
	return s
}

// EventsEndpoint implements config.RuntimeConfig interface.
func (s *EventSinkV1Alpha1) EventsEndpoint() *string {
	return pointer.To(s.Endpoint)
}

// KmsgLogURLs implements config.RuntimeConfig interface.
func (s *EventSinkV1Alpha1) KmsgLogURLs() []*url.URL {
	return nil
}

// WatchdogTimer implements config.RuntimeConfig interface.
func (s *EventSinkV1Alpha1) WatchdogTimer() config.WatchdogTimerConfig {
	return nil
}

// Validate implements config.Validator interface.
func (s *EventSinkV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	_, _, err := net.SplitHostPort(s.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("event sink endpoint: %w", err)
	}

	return nil, nil
}
