// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvm

import (
	"context"
	"fmt"
	"strings"
)

// UdevKeyVGNameComplete is the udev key emitted by `pvscan ... --udevoutput`
// when every PV backing a VG is online; its value is the VG name ready for
// auto-activation. See lvmautoactivation(7).
const UdevKeyVGNameComplete = "LVM_VG_NAME_COMPLETE"

// VGChangeActivate activates the given volume group via
// `lvm vgchange -aay --autoactivation event <vg>`.
//
// The event auto-activation mode is what udev rules invoke; using it from a
// controller mirrors the udev path and keeps locking semantics consistent.
func (lvm *LVM) VGChangeActivate(ctx context.Context, vgName string) error {
	if _, err := lvm.run(ctx, "vgchange", "-aay", "--autoactivation", "event", vgName); err != nil {
		return fmt.Errorf("activate volume group %q: %w", vgName, err)
	}

	return nil
}

// PVScanAutoActivation runs `lvm pvscan --cache --listvg --checkcomplete
// --vgonline --autoactivation event --udevoutput <devicePath>` and returns the
// parsed udev-style key/value pairs.
//
// See lvmautoactivation(7) for the protocol. The caller typically looks for
// LVM_VG_NAME_COMPLETE to learn which VG (if any) is now fully assembled and
// ready for activation.
func (lvm *LVM) PVScanAutoActivation(ctx context.Context, devicePath string) (map[string]string, error) {
	out, err := lvm.run(
		ctx,
		"pvscan",
		"--cache",
		"--listvg",
		"--checkcomplete",
		"--vgonline",
		"--autoactivation", "event",
		"--udevoutput",
		devicePath,
	)
	if err != nil {
		return nil, fmt.Errorf("pvscan %q: %w", devicePath, err)
	}

	return parseUdevOutput(out), nil
}

// parseUdevOutput parses `KEY=VALUE` udev-style output into a map. Values may
// be quoted with single or double quotes; quotes are stripped. Lines without
// `=` are ignored.
func parseUdevOutput(out string) map[string]string {
	result := map[string]string{}

	for line := range strings.SplitSeq(out, "\n") {
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		result[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(value), "'\"")
	}

	return result
}
