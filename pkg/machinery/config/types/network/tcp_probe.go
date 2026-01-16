// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// TCPProbeKind is a TCPProbe config document kind.
const TCPProbeKind = "TCPProbeConfig"

func init() {
	registry.Register(TCPProbeKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &TCPProbeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NamedDocument         = &TCPProbeConfigV1Alpha1{}
	_ config.Validator             = &TCPProbeConfigV1Alpha1{}
	_ config.NetworkTCPProbeConfig = &TCPProbeConfigV1Alpha1{}
)

// TCPProbeConfigV1Alpha1 is a config document to configure network TCP connectivity probes.
//
//	examples:
//	  - value: exampleTCPProbeConfigV1Alpha1()
//	alias: TCPProbeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/TCPProbeConfig
type TCPProbeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the probe.
	//   examples:
	//    - value: >
	//       "proxy-check"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//nolint:embeddedstructfieldcheck
	CommonProbeConfig `yaml:",inline"`
	//   description: |
	//     Endpoint to probe in the format host:port.
	//   examples:
	//    - value: >
	//       "proxy.example.com:3128"
	//   schemaRequired: true
	TCPEndpoint string `yaml:"endpoint"`
	//   description: |
	//     Timeout for the probe.
	//     Defaults to 10s.
	//   examples:
	//    - value: >
	//       10 * time.Second
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	TCPTimeout time.Duration `yaml:"timeout,omitempty"`
}

// CommonProbeConfig holds fields common to all probe types.
type CommonProbeConfig struct {
	//   description: |
	//     Interval between probe attempts.
	//     Defaults to 1s.
	//   examples:
	//    - value: >
	//       time.Second
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	ProbeInterval time.Duration `yaml:"interval,omitempty"`
	//   description: |
	//     Number of consecutive failures for the probe to be considered failed after having succeeded.
	//     Defaults to 0 (immediately fail on first failure).
	//   examples:
	//    - value: >
	//       3
	ProbeFailureThreshold int `yaml:"failureThreshold,omitempty"`
}

// NewTCPProbeConfigV1Alpha1 creates a new TCPProbeConfigV1Alpha1 config document.
func NewTCPProbeConfigV1Alpha1(name string) *TCPProbeConfigV1Alpha1 {
	return &TCPProbeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       TCPProbeKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

// Name implements config.NamedDocument interface.
func (p *TCPProbeConfigV1Alpha1) Name() string {
	return p.MetaName
}

// Clone implements config.Document interface.
func (p *TCPProbeConfigV1Alpha1) Clone() config.Document {
	return p.DeepCopy()
}

// Validate implements config.Validator interface.
func (p *TCPProbeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		errs     error
		warnings []string //nolint:prealloc
	)

	if p.MetaName == "" {
		errs = errors.Join(errs, errors.New("probe name is required"))
	}

	if p.TCPEndpoint == "" {
		errs = errors.Join(errs, errors.New("TCP probe endpoint is required"))
	}

	if p.TCPTimeout < 0 {
		errs = errors.Join(errs, fmt.Errorf("TCP probe timeout cannot be negative: %s", p.TCPTimeout))
	}

	extraWarnings, extraErrs := p.CommonProbeConfig.Validate()
	errs = errors.Join(errs, extraErrs)

	warnings = append(warnings, extraWarnings...)

	return warnings, errs
}

func exampleTCPProbeConfigV1Alpha1() *TCPProbeConfigV1Alpha1 {
	cfg := NewTCPProbeConfigV1Alpha1("proxy-check")

	cfg.CommonProbeConfig = CommonProbeConfig{
		ProbeInterval:         time.Second,
		ProbeFailureThreshold: 3,
	}
	cfg.TCPEndpoint = "proxy.example.com:3128"
	cfg.TCPTimeout = 10 * time.Second

	return cfg
}

// Interval implements config.NetworkCommonProbeConfig interface.
func (p *CommonProbeConfig) Interval() time.Duration {
	if p.ProbeInterval == 0 {
		return time.Second
	}

	return p.ProbeInterval
}

// FailureThreshold implements config.NetworkCommonProbeConfig interface.
func (p *CommonProbeConfig) FailureThreshold() int {
	return p.ProbeFailureThreshold
}

// Validate the common probe config.
func (p *CommonProbeConfig) Validate() ([]string, error) {
	var errs error

	if p.ProbeInterval < 0 {
		errs = errors.Join(errs, fmt.Errorf("probe interval cannot be negative: %s", p.ProbeInterval))
	}

	if p.ProbeFailureThreshold < 0 {
		errs = errors.Join(errs, fmt.Errorf("probe failure threshold cannot be negative: %d", p.ProbeFailureThreshold))
	}

	return nil, errs
}

// Endpoint implements config.NetworkTCPProbeConfig interface.
func (p *TCPProbeConfigV1Alpha1) Endpoint() string {
	return p.TCPEndpoint
}

// Timeout implements config.NetworkTCPProbeConfig interface.
func (p *TCPProbeConfigV1Alpha1) Timeout() time.Duration {
	if p.TCPTimeout == 0 {
		return 10 * time.Second
	}

	return p.TCPTimeout
}
