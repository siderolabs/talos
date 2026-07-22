// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package volumeconfig_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/volumes/volumeconfig"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestResolveScrub(t *testing.T) {
	t.Parallel()

	const (
		globalInterval = 24 * time.Hour
		customInterval = 30 * time.Minute
	)

	globalDoc := func(mutate func(*blockcfg.FilesystemScrubConfigV1Alpha1)) *blockcfg.FilesystemScrubConfigV1Alpha1 {
		doc := blockcfg.NewFilesystemScrubConfigV1Alpha1()

		if mutate != nil {
			mutate(doc)
		}

		return doc
	}

	userVolume := func(name string, scrub *blockcfg.ScrubConfig) *blockcfg.UserVolumeConfigV1Alpha1 {
		doc := blockcfg.NewUserVolumeConfigV1Alpha1()
		doc.MetaName = name
		doc.ScrubSpec = scrub

		return doc
	}

	for _, tc := range []struct {
		name      string
		docs      []configconfig.Document
		volumeCfg configconfig.VolumeScrubConfigProvider

		expectedEnabled  bool
		expectedInterval time.Duration
	}{
		{
			name:      "no config defaults to enabled",
			volumeCfg: nil,

			expectedEnabled:  true,
			expectedInterval: constants.DefaultFilesystemScrubInterval,
		},
		{
			name: "global interval override",
			docs: []configconfig.Document{globalDoc(func(doc *blockcfg.FilesystemScrubConfigV1Alpha1) {
				doc.ScrubInterval = globalInterval
			})},
			volumeCfg: nil,

			expectedEnabled:  true,
			expectedInterval: globalInterval,
		},
		{
			name: "global disable",
			docs: []configconfig.Document{globalDoc(func(doc *blockcfg.FilesystemScrubConfigV1Alpha1) {
				doc.ScrubEnabled = new(false)
			})},
			volumeCfg: nil,
		},
		{
			name: "per-volume interval override",
			docs: []configconfig.Document{globalDoc(func(doc *blockcfg.FilesystemScrubConfigV1Alpha1) {
				doc.ScrubInterval = globalInterval
			})},
			volumeCfg: userVolume("data", &blockcfg.ScrubConfig{ScrubInterval: customInterval}),

			expectedEnabled:  true,
			expectedInterval: customInterval,
		},
		{
			name:      "per-volume disable overrides default",
			volumeCfg: userVolume("data", &blockcfg.ScrubConfig{ScrubEnabled: new(false)}),
		},
		{
			name: "per-volume enable overrides global disable",
			docs: []configconfig.Document{globalDoc(func(doc *blockcfg.FilesystemScrubConfigV1Alpha1) {
				doc.ScrubEnabled = new(false)
				doc.ScrubInterval = globalInterval
			})},
			volumeCfg: userVolume("data", &blockcfg.ScrubConfig{}),

			expectedEnabled:  true,
			expectedInterval: globalInterval,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tc.docs...)
			require.NoError(t, err)

			enabled, interval := volumeconfig.ResolveScrub(ctr, tc.volumeCfg)

			assert.Equal(t, tc.expectedEnabled, enabled)
			assert.Equal(t, tc.expectedInterval, interval)
		})
	}
}
