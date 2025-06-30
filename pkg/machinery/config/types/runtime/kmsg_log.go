// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"
	"net/url"

	"github.com/siderolabs/gen/ensure"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// KmsgLogKind is a kmsg log config document kind.
const KmsgLogKind = "KmsgLogConfig"

func init() {
	registry.Register(KmsgLogKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &KmsgLogV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.RuntimeConfig = &KmsgLogV1Alpha1{}
	_ config.NamedDocument = &KmsgLogV1Alpha1{}
	_ config.Validator     = &KmsgLogV1Alpha1{}
)

// KmsgLogV1Alpha1 is a event sink config document.
//
//	examples:
//	  - value: exampleKmsgLogV1Alpha1()
//	alias: KmsgLogConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/KmsgLogConfig
type KmsgLogV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the config document.
	MetaName string `yaml:"name"`
	//   description: |
	//     The URL encodes the log destination.
	//     The scheme must be tcp:// or udp://.
	//     The path must be empty.
	//     The port is required.
	//   examples:
	//     - value: >
	//        "udp://10.3.7.3:2810"
	//   schema:
	//     type: string
	//     pattern: "^(tcp|udp)://"
	KmsgLogURL meta.URL `yaml:"url"`
}

// NewKmsgLogV1Alpha1 creates a new eventsink config document.
func NewKmsgLogV1Alpha1() *KmsgLogV1Alpha1 {
	return &KmsgLogV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       KmsgLogKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleKmsgLogV1Alpha1() *KmsgLogV1Alpha1 {
	cfg := NewKmsgLogV1Alpha1()
	cfg.MetaName = "remote-log"
	cfg.KmsgLogURL.URL = ensure.Value(url.Parse("tcp://192.168.3.7:3478/"))

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *KmsgLogV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *KmsgLogV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Runtime implements config.Config interface.
func (s *KmsgLogV1Alpha1) Runtime() config.RuntimeConfig {
	return s
}

// EventsEndpoint implements config.RuntimeConfig interface.
func (s *KmsgLogV1Alpha1) EventsEndpoint() *string {
	return nil
}

// KmsgLogURLs implements config.RuntimeConfig interface.
func (s *KmsgLogV1Alpha1) KmsgLogURLs() []*url.URL {
	return []*url.URL{s.KmsgLogURL.URL}
}

// WatchdogTimer implements config.RuntimeConfig interface.
func (s *KmsgLogV1Alpha1) WatchdogTimer() config.WatchdogTimerConfig {
	return nil
}

// Validate implements config.Validator interface.
func (s *KmsgLogV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.MetaName == "" {
		return nil, errors.New("name is required")
	}

	if s.KmsgLogURL.URL == nil {
		return nil, errors.New("url is required")
	}

	switch s.KmsgLogURL.URL.Scheme {
	case "tcp":
	case "udp":
	default:
		return nil, errors.New("url scheme must be tcp:// or udp://")
	}

	switch s.KmsgLogURL.URL.Path {
	case "/":
	case "":
	default:
		return nil, errors.New("url path must be empty")
	}

	if s.KmsgLogURL.URL.Port() == "" {
		return nil, errors.New("url port is required")
	}

	return nil, nil
}
