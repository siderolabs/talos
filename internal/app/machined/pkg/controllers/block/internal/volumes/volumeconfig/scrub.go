// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig

import (
	"time"

	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ResolveScrub resolves the effective scrub configuration for a volume by combining the global
// FilesystemScrubConfig with the per-volume scrub override.
//
// Scrubbing is disabled by default with the default interval; the global document adjusts or
// enables it for all volumes, and the per-volume override takes precedence over both.
//
// It returns enabled=false (and interval=0) when scrubbing is disabled for the volume.
func ResolveScrub(cfg configconfig.Config, scrubConfig configconfig.VolumeScrubConfigProvider) (enabled bool, interval time.Duration) {
	if cfg == nil {
		return false, 0
	}

	interval = 0

	if globalScrub := cfg.FilesystemScrubConfig(); globalScrub != nil {
		interval = globalScrub.Interval().ValueOr(0)
	}

	enabled = interval > 0

	if scrubConfig != nil {
		if volumeScrub := scrubConfig.Scrub(); volumeScrub != nil {
			// a per-volume scrub block enables scrubbing unless explicitly disabled.
			enabled = volumeScrub.Enabled().ValueOr(true)

			if v, ok := volumeScrub.Interval().Get(); ok {
				interval = v
			} else if interval == 0 {
				interval = constants.DefaultFilesystemScrubInterval
			}
		}
	}

	if !enabled || interval <= 0 {
		return false, 0
	}

	return true, interval
}
