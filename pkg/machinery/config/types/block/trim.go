// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"time"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
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
	//   description: |
	//     The size of the filesystem range trimmed at once, overriding the global chunk size.
	//
	//     Setting it explicitly to zero trims the whole filesystem at once.
	//     When set to a non-zero value, the chunk size must be at least 1MiB.
	//
	//     Size is specified in bytes, but can be expressed in human readable format, e.g. 1GiB.
	//   examples:
	//     - value: >
	//         "1GiB"
	//   schema:
	//     type: string
	TrimChunkSize meta.ByteSize `yaml:"chunkSize,omitempty"`
	//   description: |
	//     The delay between trimming consecutive chunks, overriding the global chunk delay.
	//
	//     Setting it explicitly to zero trims the chunks back-to-back.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	TrimChunkDelay *time.Duration `yaml:"chunkDelay,omitempty"`
	//   description: |
	//     The minimum contiguous free range to discard, overriding the global minimum length.
	//
	//     The value cannot exceed 128MiB, as ext4 rejects values larger than the block group size.
	//
	//     Size is specified in bytes, but can be expressed in human readable format, e.g. 1MiB.
	//   examples:
	//     - value: >
	//         "1MiB"
	//   schema:
	//     type: string
	TrimMinLength meta.ByteSize `yaml:"minLength,omitempty"`
}

// Validate the trim configuration.
func (t *TrimConfig) Validate() error {
	if t == nil {
		return nil
	}

	if t.TrimInterval < 0 {
		return errors.New("trim interval cannot be negative")
	}

	if t.TrimChunkDelay != nil && *t.TrimChunkDelay < 0 {
		return errors.New("trim chunk delay cannot be negative")
	}

	if err := validateTrimSizes(t.TrimChunkSize, t.TrimMinLength); err != nil {
		return fmt.Errorf("trim %w", err)
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

// ChunkSize implements config.VolumeTrimConfig interface.
func (t *TrimConfig) ChunkSize() optional.Optional[uint64] {
	if t == nil || t.TrimChunkSize.IsZero() {
		return optional.None[uint64]()
	}

	return optional.Some(t.TrimChunkSize.Value())
}

// ChunkDelay implements config.VolumeTrimConfig interface.
func (t *TrimConfig) ChunkDelay() optional.Optional[time.Duration] {
	if t == nil || t.TrimChunkDelay == nil {
		return optional.None[time.Duration]()
	}

	return optional.Some(*t.TrimChunkDelay)
}

// MinLength implements config.VolumeTrimConfig interface.
func (t *TrimConfig) MinLength() optional.Optional[uint64] {
	if t == nil || t.TrimMinLength.IsZero() {
		return optional.None[uint64]()
	}

	return optional.Some(t.TrimMinLength.Value())
}
