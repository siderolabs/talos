// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"time"

	"github.com/siderolabs/gen/optional"
)

// TrimConfig describes per-volume filesystem trim (fstrim) configuration.
//
// It overrides the global FilesystemTrimConfig for the volume.
type TrimConfig struct {
	//   description: |
	//     Enable or disable trimming for this volume.
	//
	//     If not set, trimming is enabled when the global FilesystemTrimConfig is present.
	TrimEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     The interval at which the volume is trimmed, overriding the global trim interval.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	TrimInterval time.Duration `yaml:"interval,omitempty"`
}

// Validate the trim configuration.
func (t *TrimConfig) Validate() error {
	if t == nil {
		return nil
	}

	if t.TrimInterval < 0 {
		return errors.New("trim interval cannot be negative")
	}

	return nil
}

// Enabled implements config.VolumeTrimConfig interface.
func (t *TrimConfig) Enabled() optional.Optional[bool] {
	if t == nil || t.TrimEnabled == nil {
		return optional.None[bool]()
	}

	return optional.Some(*t.TrimEnabled)
}

// Interval implements config.VolumeTrimConfig interface.
func (t *TrimConfig) Interval() optional.Optional[time.Duration] {
	if t == nil || t.TrimInterval == 0 {
		return optional.None[time.Duration]()
	}

	return optional.Some(t.TrimInterval)
}
