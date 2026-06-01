// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions_test

import (
	"path/filepath"
	"testing"

	"github.com/blang/semver/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/extensions"
)

func TestLoadValidate(t *testing.T) {
	ext, err := extensions.Load("testdata/good/extension1")
	require.NoError(t, err)

	assert.Equal(t, "gvisor", ext.Manifest.Metadata.Name)

	version, err := semver.Parse("1.0.0")
	require.NoError(t, err)

	assert.NoError(t, ext.Validate(
		extensions.WithValidateConstraints(),
		extensions.WithValidateContents(),
		extensions.WithTalosVersion(&version),
	))
}

func TestValidateFailures(t *testing.T) {
	version, err := semver.Parse("1.0.0")
	require.NoError(t, err)

	for _, tt := range []struct {
		name          string
		loadError     string
		validateError string
	}{
		{
			name:      "wrongfiles",
			loadError: "unexpected file \"a\"",
		},
		{
			name:      "emptymanifest",
			loadError: "unsupported manifest version: \"\"",
		},
		{
			name:      "norootfs",
			loadError: "extension rootfs is missing",
		},
		{
			name:          "badpaths",
			validateError: "path \"/boot/vmlinuz\" is not allowed in extensions",
		},
		{
			name:          "usrmerge",
			validateError: "path \"/usr/lib64/a.so\" is not allowed in extensions",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			ext, err := extensions.Load(filepath.Join("testdata/bad", tt.name))

			if tt.loadError == "" {
				require.NoError(t, err)
			} else {
				assert.EqualError(t, err, tt.loadError)
			}

			if err == nil {
				err = ext.Validate(
					extensions.WithValidateConstraints(),
					extensions.WithValidateContents(),
					extensions.WithTalosVersion(&version),
				)

				assert.EqualError(t, err, tt.validateError)
			}
		})
	}
}
