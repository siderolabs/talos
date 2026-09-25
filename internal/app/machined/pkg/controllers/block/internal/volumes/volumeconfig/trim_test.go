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
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
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

	globalDocWithOptions := func() *blockcfg.FilesystemTrimConfigV1Alpha1 {
		doc := globalDoc()
		doc.TrimChunkSize = blockcfg.MustByteSize("1GiB")
		doc.TrimChunkDelay = 250 * time.Millisecond
		doc.TrimMinLength = blockcfg.MustByteSize("1MiB")

		return doc
	}

	globalOptions := block.TrimOptionsSpec{
		ChunkSize:  1024 * 1024 * 1024,
		ChunkDelay: 250 * time.Millisecond,
		MinLength:  1024 * 1024,
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
		expectedOptions  block.TrimOptionsSpec
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
			name:             "global options",
			docs:             []configconfig.Document{globalDocWithOptions()},
			volumeCfg:        userVolume("data", &blockcfg.TrimConfig{TrimInterval: customInterval}),
			expectedEnabled:  true,
			expectedInterval: customInterval,
			expectedOptions:  globalOptions,
		},
		{
			name: "per-volume options override",
			docs: []configconfig.Document{globalDocWithOptions()},
			volumeCfg: userVolume("data", &blockcfg.TrimConfig{
				TrimChunkSize:  blockcfg.MustByteSize("0"),
				TrimChunkDelay: new(time.Second),
				TrimMinLength:  blockcfg.MustByteSize("4MiB"),
			}),
			expectedEnabled:  true,
			expectedInterval: globalInterval,
			expectedOptions: block.TrimOptionsSpec{
				ChunkSize:  0,
				ChunkDelay: time.Second,
				MinLength:  4 * 1024 * 1024,
			},
		},
		{
			name: "per-volume zero chunk delay overrides global",
			docs: []configconfig.Document{globalDocWithOptions()},
			volumeCfg: userVolume("data", &blockcfg.TrimConfig{
				TrimChunkDelay: new(time.Duration(0)),
			}),
			expectedEnabled:  true,
			expectedInterval: globalInterval,
			expectedOptions: block.TrimOptionsSpec{
				ChunkSize:  globalOptions.ChunkSize,
				ChunkDelay: 0,
				MinLength:  globalOptions.MinLength,
			},
		},
		{
			name: "per-volume options without global",
			volumeCfg: userVolume("data", &blockcfg.TrimConfig{
				TrimInterval:  customInterval,
				TrimChunkSize: blockcfg.MustByteSize("512MiB"),
			}),
			expectedEnabled:  true,
			expectedInterval: customInterval,
			expectedOptions: block.TrimOptionsSpec{
				ChunkSize: 512 * 1024 * 1024,
			},
		},
		{
			name:      "disabled with global options",
			docs:      []configconfig.Document{globalDocWithOptions()},
			volumeCfg: userVolume("data", &blockcfg.TrimConfig{TrimEnabled: new(false)}),
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

			enabled, interval, options := volumeconfig.ResolveTrim(ctr, tc.volumeCfg)

			assert.Equal(t, tc.expectedEnabled, enabled)
			assert.Equal(t, tc.expectedInterval, interval)
			assert.Equal(t, tc.expectedOptions, options)
		})
	}
}
