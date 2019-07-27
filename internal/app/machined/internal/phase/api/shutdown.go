/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package api

import (
	"log"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"github.com/pkg/errors"
)

const (
	// See https://github.com/torvalds/linux/blob/master/drivers/acpi/event.c
	acpiGenlFamilyName     = "acpi_event"
	acpiGenlMcastGroupName = "acpi_mc_group"
)

func listenForPowerButton() (poweroffCh <-chan struct{}, err error) {
	// Get the acpi_event family.

	conn, err := genetlink.Dial(nil)
	if err != nil {
		return nil, err
	}

	f, err := conn.GetFamily(acpiGenlFamilyName)
	if netlink.IsNotExist(err) {
		// nolint: errcheck
		conn.Close()
		return nil, errors.Wrap(err, acpiGenlFamilyName+" not available")
	}

	var id uint32
	for _, group := range f.Groups {
		if group.Name == acpiGenlMcastGroupName {
			id = group.ID
		}
	}
	if err = conn.JoinGroup(id); err != nil {
		// nolint: errcheck
		conn.Close()
		return nil, err
	}

	// Listen for ACPI events.
	ch := make(chan struct{})

	go func() {
		// nolint: errcheck
		defer conn.Close()

		for {
			msgs, _, err := conn.Receive()
			if err != nil {
				log.Printf("error reading from ACPI channel: %s", err)
				return
			}
			if len(msgs) > 0 {
				close(ch)
				return
			}
		}
	}()

	return ch, nil
}
