// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/siderolabs/gen/ensure"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// HTTPProbeKind is a HTTPProbe config document kind.
const HTTPProbeKind = "HTTPProbeConfig"

func init() {
	registry.Register(HTTPProbeKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &HTTPProbeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NamedDocument          = &HTTPProbeConfigV1Alpha1{}
	_ config.Validator              = &HTTPProbeConfigV1Alpha1{}
	_ config.NetworkHTTPProbeConfig = &HTTPProbeConfigV1Alpha1{}
)

// HTTPProbeConfigV1Alpha1 is a config document to configure network HTTP connectivity probes.
//
//	examples:
//	  - value: exampleHTTPProbeConfigV1Alpha1()
//	alias: HTTPProbeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/HTTPProbeConfig
type HTTPProbeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the probe.
	//   examples:
	//    - value: >
	//       "http-check"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//nolint:embeddedstructfieldcheck
	CommonProbeConfig `yaml:",inline"`
	//   description: |
	//     HTTP or HTTPS URL to probe. The probe succeeds if the server responds with a 2xx or 3xx status code.
	//     Probe does not follow redirects.
	//   examples:
	//    - value: >
	//       "https://example.com"
	//   schema:
	//     type: string
	//     pattern: "^(http|https)://"
	//   schemaRequired: true
	HTTPEndpoint meta.URL `yaml:"url"`
	//   description: |
	//     Timeout for the probe.
	//     Defaults to 10s.
	//   examples:
	//    - value: >
	//       10 * time.Second
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	HTTPTimeout time.Duration `yaml:"timeout,omitempty"`
}

// NewHTTPProbeConfigV1Alpha1 creates a new HTTPProbeConfigV1Alpha1 config document.
func NewHTTPProbeConfigV1Alpha1(name string) *HTTPProbeConfigV1Alpha1 {
	return &HTTPProbeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       HTTPProbeKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

// Name implements config.NamedDocument interface.
func (p *HTTPProbeConfigV1Alpha1) Name() string {
	return p.MetaName
}

// Clone implements config.Document interface.
func (p *HTTPProbeConfigV1Alpha1) Clone() config.Document {
	return p.DeepCopy()
}

// Validate implements config.Validator interface.
func (p *HTTPProbeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	if p.MetaName == "" {
		errs = errors.Join(errs, errors.New("probe name is required"))
	}

	if p.HTTPEndpoint.URL == nil {
		errs = errors.Join(errs, errors.New("HTTP probe URL is required"))
	} else {
		if p.HTTPEndpoint.URL.Scheme != "http" && p.HTTPEndpoint.URL.Scheme != "https" {
			errs = errors.Join(errs, fmt.Errorf("HTTP probe URL scheme must be http or https, got %q", p.HTTPEndpoint.URL.Scheme))
		} else if p.HTTPEndpoint.URL.Opaque != "" || p.HTTPEndpoint.URL.Hostname() == "" {
			errs = errors.Join(errs, errors.New("HTTP probe URL must be an absolute http or https URL with a non-empty host"))
		}
	}

	if p.HTTPTimeout < 0 {
		errs = errors.Join(errs, fmt.Errorf("HTTP probe timeout cannot be negative: %s", p.HTTPTimeout))
	}

	extraWarnings, extraErrs := p.CommonProbeConfig.Validate()
	errs = errors.Join(errs, extraErrs)

	warnings = append(warnings, extraWarnings...)

	return warnings, errs
}

func exampleHTTPProbeConfigV1Alpha1() *HTTPProbeConfigV1Alpha1 {
	cfg := NewHTTPProbeConfigV1Alpha1("http-check")

	cfg.CommonProbeConfig = CommonProbeConfig{
		ProbeInterval:         time.Second,
		ProbeFailureThreshold: 3,
	}
	cfg.HTTPEndpoint.URL = ensure.Value(url.Parse("https://example.com"))
	cfg.HTTPTimeout = 10 * time.Second

	return cfg
}

// URL implements config.NetworkHTTPProbeConfig interface.
func (p *HTTPProbeConfigV1Alpha1) URL() meta.URL {
	return p.HTTPEndpoint
}

// Timeout implements config.NetworkHTTPProbeConfig interface.
func (p *HTTPProbeConfigV1Alpha1) Timeout() time.Duration {
	if p.HTTPTimeout == 0 {
		return 10 * time.Second
	}

	return p.HTTPTimeout
}
