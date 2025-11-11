// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

//docgen:jsonschema

import (
	"errors"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// TimeSyncKind is a TimeSyncConfig document kind.
const TimeSyncKind = "TimeSyncConfig"

func init() {
	registry.Register(TimeSyncKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &TimeSyncConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.NetworkTimeSyncConfig        = &TimeSyncConfigV1Alpha1{}
	_ config.Validator                    = &TimeSyncConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &TimeSyncConfigV1Alpha1{}
)

// TimeSyncConfigV1Alpha1 is a config document to configure time synchronization (NTP).
//
//	examples:
//	  - value: exampleTimeSyncConfigV1Alpha1()
//	  - value: exampleTimeSyncConfigV1Alpha2()
//	alias: TimeSyncConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/TimeSyncConfig
type TimeSyncConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Indicates if the time synchronization is enabled for the machine.
	//     Defaults to `true`.
	TimeEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     Specifies the timeout when the node time is considered to be in sync unlocking the boot sequence.
	//     NTP sync will be still running in the background.
	//     Defaults to "infinity" (waiting forever for time sync)
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	TimeBootTimeout time.Duration `yaml:"bootTimeout,omitempty"`
	//   description: |
	//     Specifies NTP configuration to sync the time over network.
	//     Mutually exclusive with PTP configuration.
	TimeNTP *NTPConfig `yaml:"ntp,omitempty"`
	//   description: |
	//     Specific PTP (Precision Time Protocol) configuration to sync the time over PTP devices.
	//     Mutually exclusive with NTP configuration.
	TimePTP *PTPConfig `yaml:"ptp,omitempty"`
}

// NTPConfig represents a NTP server configuration.
type NTPConfig struct {
	//   description: |
	//     Specifies time (NTP) servers to use for setting the system time.
	//     Defaults to `time.cloudflare.com`.
	Servers []string `yaml:"servers,omitempty"`
}

// PTPConfig represents a PTP (Precision Time Protocol) configuration.
type PTPConfig struct {
	//   description: |
	//     A list of PTP devices to sync with (e.g. provided by the hypervisor).
	//
	//     A PTP device is typically represented as a character device file in the /dev directory,
	//	   such as /dev/ptp0 or /dev/ptp_kvm. These devices are used to synchronize the system time
	//     with an external time source that supports the Precision Time Protocol.
	Devices []string `yaml:"devices,omitempty"`
}

// NewTimeSyncConfigV1Alpha1 creates a new TimeSyncConfig config document.
func NewTimeSyncConfigV1Alpha1() *TimeSyncConfigV1Alpha1 {
	return &TimeSyncConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       TimeSyncKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleTimeSyncConfigV1Alpha1() *TimeSyncConfigV1Alpha1 {
	cfg := NewTimeSyncConfigV1Alpha1()
	cfg.TimeNTP = &NTPConfig{
		Servers: []string{"pool.ntp.org"},
	}

	return cfg
}

func exampleTimeSyncConfigV1Alpha2() *TimeSyncConfigV1Alpha1 {
	cfg := NewTimeSyncConfigV1Alpha1()
	cfg.TimePTP = &PTPConfig{
		Devices: []string{"/dev/ptp0"},
	}

	return cfg
}

// Clone implements config.Document interface.
func (s *TimeSyncConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *TimeSyncConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var errs error

	if s.TimeBootTimeout < 0 {
		errs = errors.Join(errs, errors.New("bootTimeout cannot be negative"))
	}

	if s.TimeNTP != nil && s.TimePTP != nil {
		errs = errors.Join(errs, errors.New("only one of ntp or ptp configuration can be specified"))
	}

	return nil, errs
}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (s *TimeSyncConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	v1tsc := v1alpha1Cfg.NetworkTimeSyncConfig()

	if v1tsc == nil {
		return nil
	}

	if v1tsc.Disabled() {
		return errors.New("time sync cannot be disabled in both v1alpha1 and new-style configuration")
	}

	if len(v1tsc.Servers()) > 0 {
		return errors.New("time servers cannot be specified in both v1alpha1 and new-style configuration")
	}

	if v1tsc.BootTimeout() != 0 {
		return errors.New("boot timeout cannot be specified in both v1alpha1 and new-style configuration")
	}

	return nil
}

// Disabled implements config.NetworkTimeSyncConfig interface.
func (s *TimeSyncConfigV1Alpha1) Disabled() bool {
	if s.TimeEnabled == nil {
		return false
	}

	return !*s.TimeEnabled
}

// BootTimeout implements config.NetworkTimeSyncConfig interface.
func (s *TimeSyncConfigV1Alpha1) BootTimeout() time.Duration {
	return s.TimeBootTimeout
}

// Servers implements config.NetworkTimeSyncConfig interface.
func (s *TimeSyncConfigV1Alpha1) Servers() []string {
	// The configuration validates that only one of the NTP or PTP is set.
	if s.TimeNTP != nil {
		return s.TimeNTP.Servers
	}

	if s.TimePTP != nil {
		return s.TimePTP.Devices
	}

	return nil
}
