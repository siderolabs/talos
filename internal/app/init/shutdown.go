/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"os"

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

	genconn, err := genetlink.Dial(nil)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer genconn.Close()
	var f genetlink.Family
	if f, err = genconn.GetFamily(acpiGenlFamilyName); os.IsNotExist(err) {
		return errors.Wrap(err, acpiGenlFamilyName+" not available")
	}

	// Listen for ACPI event.

	conn, err := netlink.Dial(unix.NETLINK_GENERIC, nil)
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer conn.Close()
	var id uint32
	for _, group := range f.Groups {
		if group.Name == acpiGenlMcastGroupName {
			id = group.ID
		}
	}
	if err = conn.JoinGroup(id); err != nil {
		return err
	}
	msgs, err := conn.Receive()
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
