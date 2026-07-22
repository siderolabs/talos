// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"fmt"
	"time"

	"github.com/siderolabs/gen/optional"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// FilesystemScrubConfigKind is a filesystem scrub config document kind.
const FilesystemScrubConfigKind = "FilesystemScrubConfig"

func init() {
	registry.Register(FilesystemScrubConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &FilesystemScrubConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.FilesystemScrubConfig = &FilesystemScrubConfigV1Alpha1{}
	_ config.Validator             = &FilesystemScrubConfigV1Alpha1{}
)

// MinScrubInterval is the minimum allowed scrub interval.
const MinScrubInterval = 10 * time.Second

// FilesystemScrubConfigV1Alpha1 is a filesystem scrub configuration document.
//
//	description: |
//	  Filesystem scrub periodically checks mounted filesystems which support online scrubbing
//	  (currently XFS, via `xfs_scrub`) for metadata errors.
//
//	  Scrubbing is enabled by default with a interval of one week; this document adjusts the default
//	  interval or disables scrubbing globally. Individual volumes can override the global settings
//	  via the `scrub` section of the volume configuration.
//
//	  Each volume is scrubbed at a stable, hash-derived time within the interval, which is different
//	  for each volume and each node, so that scrubs are spread out over time.
//	examples:
//	  - value: exampleFilesystemScrubConfigV1Alpha1()
//	alias: FilesystemScrubConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/FilesystemScrubConfig
type FilesystemScrubConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Enable or disable periodic filesystem scrubbing.
	//
	//     If not set, scrubbing is enabled by default.
	ScrubEnabled *bool `yaml:"enabled,omitempty"`
	//   description: |
	//     The interval at which the filesystems are scrubbed.
	//
	//     Default value is 1 week, minimum value is 10 seconds.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	ScrubInterval time.Duration `yaml:"interval,omitempty"`
}

// NewFilesystemScrubConfigV1Alpha1 creates a new filesystem scrub config document.
func NewFilesystemScrubConfigV1Alpha1() *FilesystemScrubConfigV1Alpha1 {
	return &FilesystemScrubConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       FilesystemScrubConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleFilesystemScrubConfigV1Alpha1() *FilesystemScrubConfigV1Alpha1 {
	cfg := NewFilesystemScrubConfigV1Alpha1()
	cfg.ScrubInterval = constants.DefaultFilesystemScrubInterval

	return cfg
}

// Clone implements config.Document interface.
func (s *FilesystemScrubConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
func (s *FilesystemScrubConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.ScrubInterval != 0 && s.ScrubInterval < MinScrubInterval {
		return nil, fmt.Errorf("scrub interval: minimum value is %s", MinScrubInterval)
	}

	return nil, nil
}

// FilesystemScrubConfigSignal is a signal for filesystem scrub config.
func (s *FilesystemScrubConfigV1Alpha1) FilesystemScrubConfigSignal() {}

// Enabled implements config.FilesystemScrubConfig interface.
func (s *FilesystemScrubConfigV1Alpha1) Enabled() optional.Optional[bool] {
	if s.ScrubEnabled == nil {
		return optional.None[bool]()
	}

	return optional.Some(*s.ScrubEnabled)
}

// Interval implements config.FilesystemScrubConfig interface.
func (s *FilesystemScrubConfigV1Alpha1) Interval() optional.Optional[time.Duration] {
	if s.ScrubInterval == 0 {
		return optional.None[time.Duration]()
	}

	return optional.Some(s.ScrubInterval)
}
