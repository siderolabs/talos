// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:dupl,goconst
package block_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
)

func TestDiskSMARTConfigMarshalUnmarshal(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		filename string
		cfg      func(t *testing.T) *block.DiskSMARTConfigV1Alpha1
	}{
		{
			name:     "full config",
			filename: "disksmartconfig_full.yaml",
			cfg: func(t *testing.T) *block.DiskSMARTConfigV1Alpha1 {
				enabled := true

				c := block.NewDiskSMARTConfigV1Alpha1()
				c.SMARTEnabled = &enabled
				c.SMARTInterval = 30 * time.Minute

				return c
			},
		},
		{
			name:     "min config",
			filename: "disksmartconfig_min.yaml",
			cfg: func(t *testing.T) *block.DiskSMARTConfigV1Alpha1 {
				c := block.NewDiskSMARTConfigV1Alpha1()

				return c
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			warnings, err := cfg.Validate(validationMode{})
			require.NoError(t, err)
			require.Empty(t, warnings)

			marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
			require.NoError(t, err)

			t.Log(string(marshaled))

			expectedMarshaled, err := os.ReadFile(filepath.Join("testdata", test.filename))
			require.NoError(t, err)

			assert.Equal(t, string(expectedMarshaled), string(marshaled))

			provider, err := configloader.NewFromBytes(expectedMarshaled)
			require.NoError(t, err)

			docs := provider.Documents()
			require.Len(t, docs, 1)

			assert.Equal(t, cfg, docs[0])
		})
	}
}

func TestDiskSMARTConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		cfg func(t *testing.T) *block.DiskSMARTConfigV1Alpha1

		expectedErrors string
	}{
		{
			name: "minimal",

			cfg: func(t *testing.T) *block.DiskSMARTConfigV1Alpha1 {
				return block.NewDiskSMARTConfigV1Alpha1()
			},
		},
		{
			name: "full",

			cfg: func(t *testing.T) *block.DiskSMARTConfigV1Alpha1 {
				enabled := false

				c := block.NewDiskSMARTConfigV1Alpha1()
				c.SMARTEnabled = &enabled
				c.SMARTInterval = 30 * time.Minute

				return c
			},
		},
		{
			name: "negative interval",

			cfg: func(t *testing.T) *block.DiskSMARTConfigV1Alpha1 {
				c := block.NewDiskSMARTConfigV1Alpha1()
				c.SMARTInterval = -time.Second

				return c
			},

			expectedErrors: "interval cannot be negative",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg(t)

			_, err := cfg.Validate(validationMode{})

			if test.expectedErrors == "" {
				require.NoError(t, err)
			} else {
				require.Error(t, err)

				assert.EqualError(t, err, test.expectedErrors)
			}
		})
	}
}
