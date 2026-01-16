// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"fmt"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// ProbeKind is a Probe config document kind.
const ProbeKind = "ProbeConfig"

func init() {
	registry.Register(ProbeKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &ProbeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NamedDocument = &ProbeConfigV1Alpha1{}
	_ config.Validator     = &ProbeConfigV1Alpha1{}
)

// ProbeConfigV1Alpha1 is a config document to configure network connectivity probes.
//
//	examples:
//	  - value: exampleProbeConfigV1Alpha1()
//	alias: ProbeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ProbeConfig
type ProbeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the probe.
	//   examples:
	//    - value: >
	//       "proxy-check"
	//   schemaRequired: true
	MetaName string `yaml:"name"`
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
	FailureThreshold int `yaml:"failureThreshold,omitempty"`
	//   description: |
	//     TCP probe configuration.
	//   schemaRequired: true
	TCP *TCPProbeConfigV1Alpha1 `yaml:"tcp,omitempty"`
}

// TCPProbeConfigV1Alpha1 describes TCP probe configuration.
type TCPProbeConfigV1Alpha1 struct {
	//   description: |
	//     Endpoint to probe in the format host:port.
	//   examples:
	//    - value: >
	//       "proxy.example.com:3128"
	//   schemaRequired: true
	Endpoint string `yaml:"endpoint"`
	//   description: |
	//     Timeout for the probe.
	//     Defaults to 10s.
	//   examples:
	//    - value: >
	//       10 * time.Second
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	Timeout time.Duration `yaml:"timeout,omitempty"`
}

// Name implements config.NamedDocument interface.
func (p *ProbeConfigV1Alpha1) Name() string {
	return p.MetaName
}

// Clone implements config.Document interface.
func (p *ProbeConfigV1Alpha1) Clone() config.Document {
	return p.DeepCopy()
}

// Validate implements config.Validator interface.
func (p *ProbeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if p.MetaName == "" {
		return nil, fmt.Errorf("probe name is required")
	}

	if p.TCP == nil {
		return nil, fmt.Errorf("probe type must be specified (currently only TCP is supported)")
	}

	if p.TCP.Endpoint == "" {
		return nil, fmt.Errorf("TCP probe endpoint is required")
	}

	// Set defaults
	if p.ProbeInterval == 0 {
		p.ProbeInterval = time.Second
	}

	if p.TCP.Timeout == 0 {
		p.TCP.Timeout = 10 * time.Second
	}

	return nil, nil
}

func exampleProbeConfigV1Alpha1() *ProbeConfigV1Alpha1 {
	return &ProbeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ProbeKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName:         "proxy-check",
		ProbeInterval:    time.Second,
		FailureThreshold: 3,
		TCP: &TCPProbeConfigV1Alpha1{
			Endpoint: "proxy.example.com:3128",
			Timeout:  10 * time.Second,
		},
	}
}
