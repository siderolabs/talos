// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package imager

import (
	"context"

	"github.com/siderolabs/talos/pkg/imager/profile"
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
