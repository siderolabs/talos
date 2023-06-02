// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package siderolink provides siderolink config documents.
package siderolink

import (
	"net/url"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
)

//go:generate deep-copy -type ConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .

// Kind is a siderolink config document kind.
const Kind = "SideroLinkConfig"

func init() {
	registry.Register(Kind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &ConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var _ config.SecretDocument = &ConfigV1Alpha1{}

// ConfigV1Alpha1 is a siderolink config document.
type ConfigV1Alpha1 struct {
	meta.Meta    `yaml:",inline"`
	APIUrlConfig meta.URL `yaml:"apiUrl"`
}

// NewConfigV1Alpha1 creates a new siderolink config document.
func NewConfigV1Alpha1() *ConfigV1Alpha1 {
	return &ConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       Kind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

// Clone implements config.Document interface.
func (s *ConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Redact implements config.SecretDocument interface.
func (s *ConfigV1Alpha1) Redact(replacement string) {
	if s.APIUrlConfig.URL != nil {
		query := s.APIUrlConfig.Query()
		if query.Has("jointoken") {
			query.Set("jointoken", replacement)
		}

		s.APIUrlConfig.RawQuery = query.Encode()
	}
}

// SideroLink implements config.SideroLink interface.
func (s *ConfigV1Alpha1) SideroLink() config.SideroLinkConfig {
	return s
}

// APIUrl implements config.SideroLink interface.
func (s *ConfigV1Alpha1) APIUrl() *url.URL {
	if s == nil {
		return nil
	}

	return s.APIUrlConfig.URL
}
