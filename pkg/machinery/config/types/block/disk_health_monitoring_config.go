// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"fmt"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// DiskHealthMonitoringConfigKind is a config document kind.
const DiskHealthMonitoringConfigKind = "DiskHealthMonitoringConfig"

func init() {
	registry.Register(DiskHealthMonitoringConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &DiskHealthMonitoringConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.DiskHealthMonitoringConfig = &DiskHealthMonitoringConfigV1Alpha1{}
	_ config.Validator                  = &DiskHealthMonitoringConfigV1Alpha1{}
)

// DiskHealthMonitoringConfigV1Alpha1 is a disk health monitoring configuration document.
//
//	description: |
//	  Configures periodic disk health monitoring. When enabled, Talos collects
//	  passive health information from NVMe and ATA disks and publishes
//	  DiskHealthStatus resources.
//	examples:
//	  - value: exampleDiskHealthMonitoringConfigV1Alpha1()
//	alias: DiskHealthMonitoringConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/DiskHealthMonitoringConfig
type DiskHealthMonitoringConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Enable or disable disk health monitoring.
	//   values:
	//     - true
	//     - false
	EnabledConfig *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     Polling interval for disk health checks.
	//     Must be a valid Go duration string (e.g. "5m", "30s", "1h").
	IntervalConfig string `yaml:"interval,omitempty"`
}

// NewDiskHealthMonitoringConfigV1Alpha1 creates a new disk health monitoring config document.
func NewDiskHealthMonitoringConfigV1Alpha1() *DiskHealthMonitoringConfigV1Alpha1 {
	return &DiskHealthMonitoringConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       DiskHealthMonitoringConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleDiskHealthMonitoringConfigV1Alpha1() *DiskHealthMonitoringConfigV1Alpha1 {
	cfg := NewDiskHealthMonitoringConfigV1Alpha1()
	enabled := true
	cfg.EnabledConfig = &enabled
	cfg.IntervalConfig = "5m"

	return cfg
}

// Clone implements config.Document interface.
func (s *DiskHealthMonitoringConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *DiskHealthMonitoringConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.IntervalConfig != "" {
		d, err := time.ParseDuration(s.IntervalConfig)
		if err != nil {
			return nil, fmt.Errorf("invalid interval %q: %w", s.IntervalConfig, err)
		}

		if d <= 0 {
			return nil, fmt.Errorf("interval must be positive, got %s", d)
		}
	}

	return nil, nil
}

// DiskHealthMonitoringConfigSignal implements config.DiskHealthMonitoringConfig interface.
func (s *DiskHealthMonitoringConfigV1Alpha1) DiskHealthMonitoringConfigSignal() {}

// DiskHealthMonitoringEnabled implements config.DiskHealthMonitoringConfig interface.
func (s *DiskHealthMonitoringConfigV1Alpha1) DiskHealthMonitoringEnabled() bool {
	if s.EnabledConfig == nil {
		return true
	}

	return *s.EnabledConfig
}

// DiskHealthMonitoringInterval implements config.DiskHealthMonitoringConfig interface.
func (s *DiskHealthMonitoringConfigV1Alpha1) DiskHealthMonitoringInterval() time.Duration {
	if s.IntervalConfig == "" {
		return 5 * time.Minute
	}

	d, err := time.ParseDuration(s.IntervalConfig)
	if err != nil {
		return 5 * time.Minute
	}

	return d
}
