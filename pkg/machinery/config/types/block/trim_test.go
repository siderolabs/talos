// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
)

func TestTrimConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg *block.TrimConfig

		expectedError string
	}{
		{
			name: "nil",
		},
		{
			name: "empty",
			cfg:  &block.TrimConfig{},
		},
		{
			name: "full",
			cfg: &block.TrimConfig{
				TrimEnabled:    new(true),
				TrimInterval:   time.Hour,
				TrimChunkSize:  block.MustByteSize("1GiB"),
				TrimChunkDelay: new(250 * time.Millisecond),
				TrimMinLength:  block.MustByteSize("1MiB"),
			},
		},
		{
			name: "explicit zeroes",
			cfg: &block.TrimConfig{
				TrimChunkSize:  block.MustByteSize("0"),
				TrimChunkDelay: new(time.Duration(0)),
				TrimMinLength:  block.MustByteSize("0"),
			},
		},
		{
			name: "negative interval",
			cfg: &block.TrimConfig{
				TrimInterval: -time.Second,
			},
			expectedError: "trim interval cannot be negative",
		},
		{
			name: "negative chunk delay",
			cfg: &block.TrimConfig{
				TrimChunkDelay: new(-time.Second),
			},
			expectedError: "trim chunk delay cannot be negative",
		},
		{
			name: "negative chunk size",
			cfg: &block.TrimConfig{
				TrimChunkSize: block.MustByteSize("-1GiB"),
			},
			expectedError: "trim chunk size cannot be negative",
		},
		{
			name: "chunk size too small",
			cfg: &block.TrimConfig{
				TrimChunkSize: block.MustByteSize("4KiB"),
			},
			expectedError: "trim chunk size cannot be less than 1.0 MiB",
		},
		{
			name: "negative minimum length",
			cfg: &block.TrimConfig{
				TrimMinLength: block.MustByteSize("-1MiB"),
			},
			expectedError: "trim minimum length cannot be negative",
		},
		{
			name: "minimum length too large",
			cfg: &block.TrimConfig{
				TrimMinLength: block.MustByteSize("129MiB"),
			},
			expectedError: "trim minimum length cannot be greater than 128 MiB",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := test.cfg.Validate()

			if test.expectedError == "" {
				require.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.expectedError)
			}
		})
	}
}

func TestTrimConfigOverrides(t *testing.T) {
	t.Parallel()

	var nilCfg *block.TrimConfig

	assert.False(t, nilCfg.Enabled().IsPresent())
	assert.False(t, nilCfg.Interval().IsPresent())
	assert.False(t, nilCfg.ChunkSize().IsPresent())
	assert.False(t, nilCfg.ChunkDelay().IsPresent())
	assert.False(t, nilCfg.MinLength().IsPresent())

	// unset fields inherit the global values
	empty := &block.TrimConfig{}

	assert.False(t, empty.Enabled().IsPresent())
	assert.False(t, empty.Interval().IsPresent())
	assert.False(t, empty.ChunkSize().IsPresent())
	assert.False(t, empty.ChunkDelay().IsPresent())
	assert.False(t, empty.MinLength().IsPresent())

	// explicit zero values override the global values
	zeroes := &block.TrimConfig{
		TrimChunkSize:  block.MustByteSize("0"),
		TrimChunkDelay: new(time.Duration(0)),
		TrimMinLength:  block.MustByteSize("0"),
	}

	assert.Equal(t, uint64(0), zeroes.ChunkSize().ValueOrZero())
	assert.True(t, zeroes.ChunkSize().IsPresent())
	assert.Equal(t, time.Duration(0), zeroes.ChunkDelay().ValueOrZero())
	assert.True(t, zeroes.ChunkDelay().IsPresent())
	assert.Equal(t, uint64(0), zeroes.MinLength().ValueOrZero())
	assert.True(t, zeroes.MinLength().IsPresent())
}

func TestTrimConfigZeroChunkDelayYAML(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes([]byte(`apiVersion: v1alpha1
kind: UserVolumeConfig
name: data
trim:
  chunkDelay: 0s
`))
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	cfg, ok := docs[0].(*block.UserVolumeConfigV1Alpha1)
	require.True(t, ok)

	require.NotNil(t, cfg.TrimSpec)

	// an explicit zero chunk delay is an override, not "unset"
	delay, present := cfg.TrimSpec.ChunkDelay().Get()
	assert.True(t, present)
	assert.Equal(t, time.Duration(0), delay)

	assert.False(t, cfg.TrimSpec.ChunkSize().IsPresent())
	assert.False(t, cfg.TrimSpec.MinLength().IsPresent())
}
