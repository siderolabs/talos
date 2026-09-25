// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	"time"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// ResolveTrim resolves the effective trim configuration for a volume by combining the global
// FilesystemTrimConfig with the per-volume trim override.
//
// It returns enabled=false (and zero interval and options) when trimming is disabled for the volume.
//
//nolint:gocyclo
func ResolveTrim(cfg configconfig.Config, trimConfig configconfig.VolumeTrimConfigProvider) (enabled bool, interval time.Duration, options block.TrimOptionsSpec) {
	if cfg == nil {
		return false, 0, block.TrimOptionsSpec{}
	}

	var globalInterval time.Duration

	if globalTrim := cfg.FilesystemTrimConfig(); globalTrim != nil {
		globalInterval = globalTrim.Interval()

		options = block.TrimOptionsSpec{
			ChunkSize:  globalTrim.ChunkSize(),
			ChunkDelay: globalTrim.ChunkDelay(),
			MinLength:  globalTrim.MinLength(),
		}
	}

	// trimming is enabled by default when the global interval is set.
	enabled = globalInterval > 0
	interval = globalInterval

	if trimConfig != nil {
		if volumeTrim := trimConfig.Trim(); volumeTrim != nil {
			// a per-volume trim block enables trimming unless explicitly disabled.
			enabled = volumeTrim.Enabled().ValueOr(true)

			if v, ok := volumeTrim.Interval().Get(); ok {
				interval = v
			}

			if v, ok := volumeTrim.ChunkSize().Get(); ok {
				options.ChunkSize = v
			}

			if v, ok := volumeTrim.ChunkDelay().Get(); ok {
				options.ChunkDelay = v
			}

			if v, ok := volumeTrim.MinLength().Get(); ok {
				options.MinLength = v
			}
		}
	}

	if !enabled || interval <= 0 {
		return false, 0, block.TrimOptionsSpec{}
	}

	return true, interval, options
}
