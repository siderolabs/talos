// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/siderolabs/gen/ensure"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// RegistryMirrorConfig defines the RegistryMirrorConfig configuration name.
const RegistryMirrorConfig = "RegistryMirrorConfig"

func init() {
	registry.Register(RegistryMirrorConfig, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &RegistryMirrorConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.RegistryMirrorConfigDocument = &RegistryMirrorConfigV1Alpha1{}
	_ config.Validator                    = &RegistryMirrorConfigV1Alpha1{}
	_ config.NamedDocument                = &RegistryMirrorConfigV1Alpha1{}
)

// RegistryMirrorConfigV1Alpha1 configures an image registry mirror.
//
//	examples:
//	  - value: exampleRegistryMirrorConfigVAlpha1()
//	alias: RegistryMirrorConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/RegistryMirrorConfig
type RegistryMirrorConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Registry name to apply the mirror configuration to.
	//
	//     Registry name is the first segment of image identifier, with 'docker.io'
	//     being default one.
	//
	//     A special name '*' can be used to define mirror configuration
	//     that applies to all registries.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     List of mirror endpoints for the registry.
	//     Mirrors will be used in the order they are specified,
	//     falling back to the default registry is `skipFallback` is not set to true.
	RegistryEndpoints []RegistryEndpoint `yaml:"endpoints,omitempty"`
	//   description: |
	//     Skip fallback to the original registry if none of the mirrors are available
	//     or contain the requested image.
	RegistrySkipFallback *bool `yaml:"skipFallback,omitempty"`
}

// RegistryEndpoint defines a registry mirror endpoint.
type RegistryEndpoint struct {
	//   description: |
	//     The URL of the registry mirror endpoint.
	//   schemaRequired: true
	//   examples:
	//     - value: >
	//        "https://my-registry-mirror.local:5000"
	//   schema:
	//     type: string
	//     pattern: "^(http|https)://"
	EndpointURL meta.URL `yaml:"url"`
	//   description: |
	//     Use endpoint path as supplied, without adding `/v2/` suffix.
	EndpointOverridePath *bool `yaml:"overridePath,omitempty"`
}

// NewRegistryMirrorConfigV1Alpha1 creates a new RegistryMirrorConfig config document.
func NewRegistryMirrorConfigV1Alpha1(name string) *RegistryMirrorConfigV1Alpha1 {
	return &RegistryMirrorConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       RegistryMirrorConfig,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleRegistryMirrorConfigVAlpha1() *RegistryMirrorConfigV1Alpha1 {
	cfg := NewRegistryMirrorConfigV1Alpha1("registry.k8s.io")
	cfg.RegistrySkipFallback = pointer.To(true)
	cfg.RegistryEndpoints = []RegistryEndpoint{
		{
			EndpointURL: meta.URL{URL: ensure.Value(url.Parse("https://my-private-registry.local:5000"))},
		},
		{
			EndpointURL:          meta.URL{URL: ensure.Value(url.Parse("http://my-harbor/v2/registry-k8s.io/"))},
			EndpointOverridePath: pointer.To(true),
		},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *RegistryMirrorConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Name implements config.Document interface.
func (s *RegistryMirrorConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Validate implements config.Validator interface.
func (s *RegistryMirrorConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string
	)

	if s.MetaName == "" {
		errs = errors.Join(errs, errors.New("name must be specified"))
	}

	for idx, ep := range s.RegistryEndpoints {
		if ep.EndpointURL.URL == nil {
			errs = errors.Join(errs, fmt.Errorf("endpoints[%d].url must be specified", idx))
		} else {
			switch ep.EndpointURL.URL.Scheme {
			case "http", "https":
			default:
				errs = errors.Join(errs, fmt.Errorf("endpoints[%d].url has unsupported scheme: %q", idx, ep.EndpointURL.URL.Scheme))
			}
		}
	}

	return warnings, errs
}

// Endpoints implements RegistryMirrorConfig interface.
func (s *RegistryMirrorConfigV1Alpha1) Endpoints() []config.RegistryEndpointConfig {
	return xslices.Map(s.RegistryEndpoints, func(ep RegistryEndpoint) config.RegistryEndpointConfig {
		return ep
	})
}

// SkipFallback implements RegistryMirrorConfig interface.
func (s *RegistryMirrorConfigV1Alpha1) SkipFallback() bool {
	return pointer.SafeDeref(s.RegistrySkipFallback)
}

// Endpoint implements RegistryEndpointConfig interface.
func (ep RegistryEndpoint) Endpoint() string {
	return ep.EndpointURL.String()
}

// OverridePath implements RegistryEndpointConfig interface.
func (ep RegistryEndpoint) OverridePath() bool {
	return pointer.SafeDeref(ep.EndpointOverridePath)
}
