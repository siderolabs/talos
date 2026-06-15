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

	return nil, nil
}

// FilesystemTrimConfigSignal is a signal for filesystem trim config.
func (s *FilesystemTrimConfigV1Alpha1) FilesystemTrimConfigSignal() {}

// Interval implements config.FilesystemTrimConfig interface.
func (s *FilesystemTrimConfigV1Alpha1) Interval() time.Duration {
	return s.TrimInterval
}
