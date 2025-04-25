// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package siderolink provides SideroLink machine configuration documents.
package siderolink

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

//go:generate docgen -output ./siderolink_doc.go ./siderolink.go

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
var (
	_ config.SecretDocument   = &ConfigV1Alpha1{}
	_ config.SideroLinkConfig = &ConfigV1Alpha1{}
	_ config.Validator        = &ConfigV1Alpha1{}
)

// ConfigV1Alpha1 is a SideroLink connection machine configuration document.
//
//	examples:
//	  - value: exampleConfigV1Alpha1()
//	alias: SideroLinkConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/SideroLinkConfig
type ConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	//   description: |
	//     SideroLink API URL to connect to.
	//   examples:
	//     - value: >
	//        "https://siderolink.api/?jointoken=secret"
	//   schema:
	//     type: string
	//     pattern: "^(https|grpc)://"
	APIUrlConfig meta.URL `yaml:"apiUrl"`
	//   description: |
	//     SideroLink unique token to use for the connection (optional).
	//
	//     This value is overridden with META key UniqueMachineToken.
	UniqueTokenConfig string `yaml:"uniqueToken,omitempty"`
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

func exampleConfigV1Alpha1() *ConfigV1Alpha1 {
	cfg := NewConfigV1Alpha1()
	cfg.APIUrlConfig.URL = ensure.Value(url.Parse("https://siderolink.api/jointoken?token=secret"))

	return cfg
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

// UniqueToken implements config.SideroLink interface.
func (s *ConfigV1Alpha1) UniqueToken() string {
	if s == nil {
		return ""
	}

	return s.UniqueTokenConfig
}

// Validate implements config.Validator interface.
func (s *ConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.APIUrlConfig.URL == nil {
		return nil, errors.New("apiUrl is required")
	}

	switch s.APIUrlConfig.URL.Scheme {
	case "https":
	case "grpc":
	default:
		return nil, errors.New("apiUrl scheme must be https:// or grpc://")
	}

	switch s.APIUrlConfig.URL.Path {
	case "/":
	case "":
	default:
		return nil, errors.New("apiUrl path must be empty")
	}

	return nil, nil
}
