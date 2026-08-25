// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/overlay"
)

// stubInstaller answers GetOptions with a fixed result; the device tree
// assembly never calls Install.
type stubInstaller struct {
	options overlay.Options
	err     error
}

func (s stubInstaller) GetOptions(context.Context, overlay.ExtraOptions) (overlay.Options, error) {
	return s.options, s.err
}

func (s stubInstaller) Install(context.Context, overlay.InstallOptions[overlay.ExtraOptions]) error {
	return nil
}

func TestOverlayDeviceTree(t *testing.T) {
	t.Parallel()

	t.Run("no overlay at all", func(t *testing.T) {
		t.Parallel()

		path, err := imager.NewForDeviceTreeTest(t.TempDir(), nil, nil).OverlayDeviceTree(t.Context())
		require.NoError(t, err)
		assert.Empty(t, path)
	})

	t.Run("overlay asking for no device tree", func(t *testing.T) {
		t.Parallel()

		installer := stubInstaller{options: overlay.Options{Name: "rock-pi-4"}}

		path, err := imager.NewForDeviceTreeTest(t.TempDir(), installer, nil).OverlayDeviceTree(t.Context())
		require.NoError(t, err)
		assert.Empty(t, path)
	})

	t.Run("base device tree with nothing to merge", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		installer := stubInstaller{
			options: overlay.Options{
				Name:       "rock-pi-4",
				DeviceTree: "rk3399-rock-pi-4c.dtb",
			},
		}

		path, err := imager.NewForDeviceTreeTest(tempDir, installer, nil).OverlayDeviceTree(t.Context())
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(tempDir, constants.ImagerOverlayArtifactsPath, "rk3399-rock-pi-4c.dtb"), path)
	})

	t.Run("failing overlay installer", func(t *testing.T) {
		t.Parallel()

		installer := stubInstaller{err: errors.New("overlay is unhappy")}

		_, err := imager.NewForDeviceTreeTest(t.TempDir(), installer, nil).OverlayDeviceTree(t.Context())
		assert.ErrorContains(t, err, "overlay is unhappy")
	})

	t.Run("overlay naming a device tree overlay which is not there", func(t *testing.T) {
		t.Parallel()

		installer := stubInstaller{
			options: overlay.Options{
				Name:               "rock-pi-4",
				DeviceTree:         "rk3399-rock-pi-4c.dtb",
				DeviceTreeOverlays: []string{"absent.dtbo"},
			},
		}

		_, err := imager.NewForDeviceTreeTest(t.TempDir(), installer, nil).OverlayDeviceTree(t.Context())
		assert.ErrorContains(t, err, "absent.dtbo")
	})
}

func TestWriteInlineDeviceTreeOverlay(t *testing.T) {
	t.Parallel()

	// A compiled overlay carries the flattened device tree magic, and has to
	// reach fdtoverlay byte for byte rather than be handed to dtc as source.
	blob := []byte{0xd0, 0x0d, 0xfe, 0xed, 0x00, 0x00, 0x00, 0x20}

	tempDir := t.TempDir()

	path, err := imager.NewForDeviceTreeTest(tempDir, nil, nil).WriteInlineDeviceTreeOverlay(t.Context(), 0, blob)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tempDir, "uki-inline-0.dtbo"), path)

	written, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, blob, written)
}
