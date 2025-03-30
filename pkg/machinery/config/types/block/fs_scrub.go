// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"fmt"
	"net/url"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// FilesystemScrubKind is a watchdog timer config document kind.
const FilesystemScrubKind = "FilesystemScrubConfig"

func init() {
	registry.Register(FilesystemScrubKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &FilesystemScrubV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.FilesystemScrubConfig = &FilesystemScrubV1Alpha1{}
	_ config.NamedDocument         = &FilesystemScrubV1Alpha1{}
	_ config.Validator             = &FilesystemScrubV1Alpha1{}
)

// Timeout constants.
const (
	MinScrubPeriod     = 10 * time.Second
	DefaultScrubPeriod = 24 * 7 * time.Hour
)

// FilesystemScrubV1Alpha1 is a filesystem scrubbing config document.
//
//	examples:
//	  - value: exampleFilesystemScrubV1Alpha1()
//	alias: FilesystemScrubConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/FilesystemScrubConfig
type FilesystemScrubV1Alpha1 struct {
	meta.Meta `yaml:",inline"`
	//   description: |
	//     Name of the config document.
	MetaName string `yaml:"name"`
	//   description: |
	//     Mountpoint of the filesystem to be scrubbed.
	//   examples:
	//     - value: >
	//        "/var"
	FSMountpoint string `yaml:"mountpoint"`
	//   description: |
	//     Period for running the scrub task for this filesystem.
	//
	//     The first run is scheduled randomly within this period from the boot time, later ones follow after the full period.
	//
	//     Default value is 1 week, minimum value is 10 seconds.
	//   schema:
	//     type: string
	//     pattern: ^[-+]?(((\d+(\.\d*)?|\d*(\.\d+)+)([nuµm]?s|m|h))|0)+$
	ScrubPeriod time.Duration `yaml:"period,omitempty"`
}

// NewFilesystemScrubV1Alpha1 creates a new eventsink config document.
func NewFilesystemScrubV1Alpha1() *FilesystemScrubV1Alpha1 {
	return &FilesystemScrubV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       FilesystemScrubKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleFilesystemScrubV1Alpha1() *FilesystemScrubV1Alpha1 {
	cfg := NewFilesystemScrubV1Alpha1()
	cfg.MetaName = "var"
	cfg.FSMountpoint = "/var"
	cfg.ScrubPeriod = 24 * 7 * time.Hour

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *FilesystemScrubV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *FilesystemScrubV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Runtime implements config.Config interface.
func (s *FilesystemScrubV1Alpha1) Runtime() config.RuntimeConfig {
	return s
}

// EventsEndpoint implements config.RuntimeConfig interface.
func (s *FilesystemScrubV1Alpha1) EventsEndpoint() *string {
	return nil
}

// KmsgLogURLs implements config.RuntimeConfig interface.
func (s *FilesystemScrubV1Alpha1) KmsgLogURLs() []*url.URL {
	return nil
}

// WatchdogTimer implements config.RuntimeConfig interface.
func (s *FilesystemScrubV1Alpha1) WatchdogTimer() config.WatchdogTimerConfig {
	return nil
}

// FilesystemScrub implements config.FilesystemScrubConfig interface.
func (s *FilesystemScrubV1Alpha1) FilesystemScrub() []config.FilesystemScrubConfig {
	return []config.FilesystemScrubConfig{s}
}

// Mountpoint implements config.FilesystemScrubConfig interface.
func (s *FilesystemScrubV1Alpha1) Mountpoint() string {
	return s.FSMountpoint
}

// Period implements config.FilesystemScrubConfig interface.
func (s *FilesystemScrubV1Alpha1) Period() time.Duration {
	if s.ScrubPeriod == 0 {
		return DefaultScrubPeriod
	}

	return s.ScrubPeriod
}

// Validate implements config.Validator interface.
func (s *FilesystemScrubV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if s.Mountpoint() == "" {
		return nil, fmt.Errorf("mountpoint: empty value")
	}

	if s.ScrubPeriod > 0 && s.ScrubPeriod < MinScrubPeriod {
		return nil, fmt.Errorf("scrub period: minimum value is %s", MinScrubPeriod)
	}

	return nil, nil
}
