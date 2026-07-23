// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// DiskSMARTConfigKind is a config document kind.
const DiskSMARTConfigKind = "DiskSMARTConfig"

func init() {
	registry.Register(DiskSMARTConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &DiskSMARTConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.DiskSMARTConfig = &DiskSMARTConfigV1Alpha1{}
	_ config.Validator       = &DiskSMARTConfigV1Alpha1{}
)

// DiskSMARTConfigV1Alpha1 is a disk SMART monitoring configuration document.
//
//	description: |
//	  Disk SMART monitoring periodically collects SMART (Self-Monitoring, Analysis and Reporting
//	  Technology) health information from disks, exposed via the `SMARTStatus` resource
//	  (`talosctl get smart`).
//
//	  SMART collection is enabled by default; this document allows tuning the refresh interval or
//	  disabling it. Disks in standby are never spun up just to be probed.
//	examples:
//	  - value: exampleDiskSMARTConfigV1Alpha1()
//	alias: DiskSMARTConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/DiskSMARTConfig
type DiskSMARTConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Enable or disable disk SMART monitoring.
	//
	//     Defaults to enabled when this document is present.
	SMARTEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     The interval at which disk SMART status is refreshed.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	SMARTInterval time.Duration `yaml:"interval,omitempty"`
}

// NewDiskSMARTConfigV1Alpha1 creates a new disk SMART config document.
func NewDiskSMARTConfigV1Alpha1() *DiskSMARTConfigV1Alpha1 {
	return &DiskSMARTConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       DiskSMARTConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleDiskSMARTConfigV1Alpha1() *DiskSMARTConfigV1Alpha1 {
	cfg := NewDiskSMARTConfigV1Alpha1()
	cfg.SMARTInterval = constants.DefaultDiskSMARTInterval

	return cfg
}

// Clone implements config.Document interface.
func (s *DiskSMARTConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *DiskSMARTConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.SMARTInterval < 0 {
		return nil, errors.New("interval cannot be negative")
	}

	return nil, nil
}

// DiskSMARTConfigSignal is a signal for disk SMART config.
func (s *DiskSMARTConfigV1Alpha1) DiskSMARTConfigSignal() {}

// Enabled implements config.DiskSMARTConfig interface.
func (s *DiskSMARTConfigV1Alpha1) Enabled() bool {
	if s.SMARTEnabled == nil {
		return true
	}

	return *s.SMARTEnabled
}

// Interval implements config.DiskSMARTConfig interface.
func (s *DiskSMARTConfigV1Alpha1) Interval() time.Duration {
	if s.SMARTInterval == 0 {
		return constants.DefaultDiskSMARTInterval
	}

	return s.SMARTInterval
}
