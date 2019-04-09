/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

const (
	// See https://github.com/torvalds/linux/blob/master/drivers/acpi/event.c
	acpiGenlFamilyName     = "acpi_event"
	acpiGenlMcastGroupName = "acpi_mc_group"
)

func listenForPowerButton() (err error) {
	// Get the acpi_event family.

	conn, err := genetlink.Dial(nil)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer conn.Close()

	f, err := conn.GetFamily(acpiGenlFamilyName)
	if netlink.IsNotExist(err) {
		return errors.Wrap(err, acpiGenlFamilyName+" not available")
	}

	var id uint32
	for _, group := range f.Groups {
		if group.Name == acpiGenlMcastGroupName {
			id = group.ID
		}
	}
	if err = conn.JoinGroup(id); err != nil {
		return err
	}

	// Listen for ACPI events.

	msgs, _, err := conn.Receive()
	if err != nil {
		return err
	}
	if len(msgs) > 0 {
		// TODO(andrewrynhard): Stop all running containerd tasks.
		// See http://man7.org/linux/man-pages/man2/reboot.2.html.
		unix.Sync()
		// nolint: errcheck
		unix.Reboot(unix.LINUX_REBOOT_CMD_POWER_OFF)
	}

	return nil
}
