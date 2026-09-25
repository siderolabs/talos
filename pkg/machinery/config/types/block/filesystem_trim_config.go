// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// FilesystemTrimConfigKind is a config document kind.
const FilesystemTrimConfigKind = "FilesystemTrimConfig"

func init() {
	registry.Register(FilesystemTrimConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &FilesystemTrimConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.FilesystemTrimConfig = &FilesystemTrimConfigV1Alpha1{}
	_ config.Validator            = &FilesystemTrimConfigV1Alpha1{}
)

// FilesystemTrimConfigV1Alpha1 is a filesystem trim (fstrim) configuration document.
//
//	description: |
//	  Filesystem trim (the equivalent of the `fstrim` command) periodically discards unused blocks
//	  of mounted filesystems which support trimming.
//
//	  When this document is present, Talos builds a stable per-node, per-volume schedule and trims
//	  eligible volumes at the configured interval. If the document is absent, no automatic trimming
//	  is performed (unless enabled explicitly on a per-volume basis).
//	examples:
//	  - value: exampleFilesystemTrimConfigV1Alpha1()
//	alias: FilesystemTrimConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/FilesystemTrimConfig
type FilesystemTrimConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The interval at which the filesystems are trimmed.
	//
	//     The trim is performed at a stable, hash-derived time within the interval, which is different
	//     for each volume and each node, so that trims are spread out over time.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	TrimInterval time.Duration `yaml:"interval,omitempty"`
	//   description: |
	//     The size of the filesystem range trimmed at once.
	//
	//     By default (or when set to zero), the whole filesystem is trimmed at once (same as the `fstrim` command).
	//     Trimming a large filesystem at once issues discards for all free space back-to-back,
	//     which might cause latency spikes for other workloads using the same disk.
	//     Setting the chunk size splits the trim into multiple operations, each covering at most
	//     the chunk size of the filesystem. When set, the chunk size must be at least 1MiB.
	//
	//     Size is specified in bytes, but can be expressed in human readable format, e.g. 1GiB.
	//   examples:
	//     - value: >
	//         "1GiB"
	//   schema:
	//     type: string
	TrimChunkSize meta.ByteSize `yaml:"chunkSize,omitempty"`
	//   description: |
	//     The delay between trimming consecutive chunks.
	//
	//     Only used when the chunk size is set.
	//   examples:
	//     - value: >
	//         "250ms"
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	TrimChunkDelay time.Duration `yaml:"chunkDelay,omitempty"`
	//   description: |
	//     The minimum contiguous free range to discard.
	//
	//     Free ranges smaller than this value are not discarded, which reduces the number of discard
	//     operations at the expense of leaving small free ranges untrimmed.
	//     The kernel raises the value to the discard granularity of the device.
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

// NewFilesystemTrimConfigV1Alpha1 creates a new filesystem trim config document.
func NewFilesystemTrimConfigV1Alpha1() *FilesystemTrimConfigV1Alpha1 {
	return &FilesystemTrimConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       FilesystemTrimConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleFilesystemTrimConfigV1Alpha1() *FilesystemTrimConfigV1Alpha1 {
	cfg := NewFilesystemTrimConfigV1Alpha1()
	cfg.TrimInterval = constants.DefaultFilesystemTrimInterval

	return cfg
}

// Clone implements config.Document interface.
func (s *FilesystemTrimConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *FilesystemTrimConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.TrimInterval < 0 {
		return nil, errors.New("interval cannot be negative")
	}

	if s.TrimChunkDelay < 0 {
		return nil, errors.New("chunk delay cannot be negative")
	}

	if err := validateTrimSizes(s.TrimChunkSize, s.TrimMinLength); err != nil {
		return nil, err
	}

	return nil, nil
}

// validateTrimSizes validates the trim chunk size and minimum length (shared by the global and per-volume configs).
func validateTrimSizes(chunkSize, minLength meta.ByteSize) error {
	if chunkSize.IsNegative() {
		return errors.New("chunk size cannot be negative")
	}

	if v := chunkSize.Value(); v != 0 && v < constants.FilesystemTrimMinChunkSize {
		return fmt.Errorf("chunk size cannot be less than %s", humanize.IBytes(constants.FilesystemTrimMinChunkSize))
	}

	if minLength.IsNegative() {
		return errors.New("minimum length cannot be negative")
	}

	if minLength.Value() > constants.FilesystemTrimMaxMinLength {
		return fmt.Errorf("minimum length cannot be greater than %s", humanize.IBytes(constants.FilesystemTrimMaxMinLength))
	}

	return nil
}

// FilesystemTrimConfigSignal is a signal for filesystem trim config.
func (s *FilesystemTrimConfigV1Alpha1) FilesystemTrimConfigSignal() {}

// Interval implements config.FilesystemTrimConfig interface.
func (s *FilesystemTrimConfigV1Alpha1) Interval() time.Duration {
	return s.TrimInterval
}

// ChunkSize implements config.FilesystemTrimConfig interface.
func (s *FilesystemTrimConfigV1Alpha1) ChunkSize() uint64 {
	return s.TrimChunkSize.Value()
}

// ChunkDelay implements config.FilesystemTrimConfig interface.
func (s *FilesystemTrimConfigV1Alpha1) ChunkDelay() time.Duration {
	return s.TrimChunkDelay
}

// MinLength implements config.FilesystemTrimConfig interface.
func (s *FilesystemTrimConfigV1Alpha1) MinLength() uint64 {
	return s.TrimMinLength.Value()
}
