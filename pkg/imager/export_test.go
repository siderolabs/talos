// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"context"

	"github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/siderolabs/talos/pkg/machinery/overlay"
	"github.com/siderolabs/talos/pkg/reporter"
)

// NewForInstallerTest builds an Imager with the boot assets already in place, skipping New().
func NewForInstallerTest(prof profile.Profile, tempDir, initramfsPath string) *Imager {
	return &Imager{
		prof:          prof,
		tempDir:       tempDir,
		initramfsPath: initramfsPath,
	}
}

// OutInstaller exposes outInstaller for tests.
func (i *Imager) OutInstaller(ctx context.Context, path string, report *reporter.Reporter) error {
	return i.outInstaller(ctx, path, report)
}

// NewForDeviceTreeTest builds an Imager with only the fields the device tree assembly reads.
func NewForDeviceTreeTest(tempDir string, installer overlay.Installer[overlay.ExtraOptions], extraOptions overlay.ExtraOptions) *Imager {
	return &Imager{
		prof: profile.Profile{
			Overlay: &profile.OverlayOptions{
				ExtraOptions: extraOptions,
			},
		},
		tempDir:          tempDir,
		overlayInstaller: installer,
	}
}

// OverlayDeviceTree exposes overlayDeviceTree for tests.
func (i *Imager) OverlayDeviceTree(ctx context.Context) (string, error) {
	return i.overlayDeviceTree(ctx)
}

// WriteInlineDeviceTreeOverlay exposes writeInlineDeviceTreeOverlay for tests.
func (i *Imager) WriteInlineDeviceTreeOverlay(ctx context.Context, idx int, blob []byte) (string, error) {
	return i.writeInlineDeviceTreeOverlay(ctx, idx, blob)
}
