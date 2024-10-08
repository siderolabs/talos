// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mount

import (
	"github.com/siderolabs/gen/xslices"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// OverlayMountPoints returns the mountpoints required to boot the system.
// These mountpoints are used as overlays on top of the read only rootfs.
func OverlayMountPoints() Points {
	return xslices.Map(constants.Overlays, func(target string) *Point {
		return NewVarOverlay([]string{target}, target, WithFlags(unix.MS_I_VERSION))
	})
}
