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
)

func TestResolveTrim(t *testing.T) {
	t.Parallel()

	const (
		globalInterval = 7 * 24 * time.Hour
		customInterval = 30 * time.Minute
	)

	globalDoc := func() *blockcfg.FilesystemTrimConfigV1Alpha1 {
		doc := blockcfg.NewFilesystemTrimConfigV1Alpha1()
		doc.TrimInterval = globalInterval

		return doc
	}

	userVolume := func(name string, trim *blockcfg.TrimConfig) *blockcfg.UserVolumeConfigV1Alpha1 {
		doc := blockcfg.NewUserVolumeConfigV1Alpha1()
		doc.MetaName = name
		doc.TrimSpec = trim

		return doc
	}

	for _, tc := range []struct {
		name      string
		docs      []configconfig.Document
		volumeCfg configconfig.VolumeTrimConfigProvider

		expectedEnabled  bool
		expectedInterval time.Duration
	}{
		{
			name:      "no config",
			volumeCfg: nil,
		},
		{
			name:             "global only",
			docs:             []configconfig.Document{globalDoc()},
			volumeCfg:        nil,
			expectedEnabled:  true,
			expectedInterval: globalInterval,
		},
		{
			name:             "per-volume interval override",
			docs:             []configconfig.Document{globalDoc()},
			volumeCfg:        userVolume("data", &blockcfg.TrimConfig{TrimInterval: customInterval}),
			expectedEnabled:  true,
			expectedInterval: customInterval,
		},
		{
			name:             "per-volume disabled overrides global",
			docs:             []configconfig.Document{globalDoc()},
			volumeCfg:        userVolume("data", &blockcfg.TrimConfig{TrimEnabled: new(false)}),
			expectedEnabled:  false,
			expectedInterval: 0,
		},
		{
			name:             "per-volume enabled without global",
			docs:             []configconfig.Document{userVolume("data", &blockcfg.TrimConfig{TrimInterval: customInterval})},
			volumeCfg:        userVolume("data", &blockcfg.TrimConfig{TrimInterval: customInterval}),
			expectedEnabled:  true,
			expectedInterval: customInterval,
		},
		{
			name:      "disabled without global is no-op",
			volumeCfg: userVolume("data", &blockcfg.TrimConfig{TrimEnabled: new(false)}),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(tc.docs...)
			require.NoError(t, err)

			enabled, interval := volumeconfig.ResolveTrim(ctr, tc.volumeCfg)

			assert.Equal(t, tc.expectedEnabled, enabled)
			assert.Equal(t, tc.expectedInterval, interval)
		})
	}
}
