// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"

	"github.com/siderolabs/go-pointer"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// ZswapConfigKind is a config document kind.
const ZswapConfigKind = "ZswapConfig"

func init() {
	registry.Register(ZswapConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &ZswapConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ZswapConfig = &ZswapConfigV1Alpha1{}
	_ config.Validator   = &ZswapConfigV1Alpha1{}
)

// ZswapConfigV1Alpha1 is a zswap (compressed memory) configuration document.
//
//	description: |
//	  When zswap is enabled, Linux kernel compresses pages that would otherwise be swapped out to disk.
//	  The compressed pages are stored in a memory pool, which is used to avoid writing to disk
//	  when the system is under memory pressure.
//	examples:
//	  - value: exampleZswapConfigV1Alpha1()
//	alias: ZswapConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ZswapConfig
type ZswapConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     The maximum percent of memory that zswap can use.
	//     This is a percentage of the total system memory.
	//     The value must be between 0 and 100.
	MaxPoolPercentConfig *int `yaml:"maxPoolPercent,omitempty"`
	//   description: |
	//    Enable the shrinker feature: kernel might move
	//    cold pages from zswap to swap device to free up memory
	//    for other use cases.
	ShrinkerEnabledConfig *bool `yaml:"shrinkerEnabled,omitempty"`
}

// NewZswapConfigV1Alpha1 creates a new zswap config document.
func NewZswapConfigV1Alpha1() *ZswapConfigV1Alpha1 {
	return &ZswapConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ZswapConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleZswapConfigV1Alpha1() *ZswapConfigV1Alpha1 {
	cfg := NewZswapConfigV1Alpha1()
	cfg.MaxPoolPercentConfig = new(25)
	cfg.ShrinkerEnabledConfig = new(true)

	return cfg
}

// Clone implements config.Document interface.
func (s *ZswapConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// Validate implements config.Validator interface.
//
//nolint:gocyclo
func (s *ZswapConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var (
		warnings         []string
		validationErrors error
	)

	if s.MaxPoolPercentConfig != nil {
		if *s.MaxPoolPercentConfig < 0 || *s.MaxPoolPercentConfig > 100 {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("maxPoolPercent must be between 0 and 100"))
		}
	}

	return warnings, validationErrors
}

// ZswapConfigSignal is a signal for zswap config.
func (s *ZswapConfigV1Alpha1) ZswapConfigSignal() {}

// MaxPoolPercent implements config.ZswapConfig interface.
func (s *ZswapConfigV1Alpha1) MaxPoolPercent() int {
	if s.MaxPoolPercentConfig == nil {
		return 20
	}

	return pointer.SafeDeref(s.MaxPoolPercentConfig)
}

// ShrinkerEnabled implements config.ZswapConfig interface.
func (s *ZswapConfigV1Alpha1) ShrinkerEnabled() bool {
	if s.ShrinkerEnabledConfig == nil {
		return false
	}

	return pointer.SafeDeref(s.ShrinkerEnabledConfig)
}
