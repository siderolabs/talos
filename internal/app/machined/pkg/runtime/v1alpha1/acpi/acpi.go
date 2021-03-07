// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package acpi

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/go-multierror"
	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
)

const (
	// PowerButtonEvent is the ACPI event name associated with the power off
	// button.
	PowerButtonEvent = "button/power"
	// See https://github.com/torvalds/linux/blob/master/drivers/acpi/event.c
	acpiGenlFamilyName     = "acpi_event"
	acpiGenlMcastGroupName = "acpi_mc_group"
)

// StartACPIListener starts listening for ACPI netlink events.
//
//nolint:gocyclo
func StartACPIListener() (err error) {
	// Get the acpi_event family.
	conn, err := genetlink.Dial(nil)
	if err != nil {
		return err
	}

	f, err := conn.GetFamily(acpiGenlFamilyName)
	if errors.Is(err, os.ErrNotExist) {
		//nolint:errcheck
		conn.Close()

		return fmt.Errorf(acpiGenlFamilyName+" not available: %w", err)
	}

	var id uint32

	for _, group := range f.Groups {
		if group.Name == acpiGenlMcastGroupName {
			id = group.ID
		}
	}

	if err = conn.JoinGroup(id); err != nil {
		//nolint:errcheck
		conn.Close()

		return err
	}

	//nolint:errcheck
	defer conn.Close()

	for {
		msgs, _, err := conn.Receive()
		if err != nil {
			return fmt.Errorf("error reading from ACPI channel: %w", err)
		}

		if len(msgs) > 0 {
			ok, err := parse(msgs, PowerButtonEvent)
			if err != nil {
				log.Printf("failed to parse netlink message: %v", err)

				continue
			}

			if !ok {
				continue
			}

			return nil
		}
	}
}

func parse(msgs []genetlink.Message, event string) (bool, error) {
	var result *multierror.Error

	for _, msg := range msgs {
		ad, err := netlink.NewAttributeDecoder(msg.Data)
		if err != nil {
			result = multierror.Append(result, fmt.Errorf("failed to create attribute decoder: %w", err))

			continue
		}

		for ad.Next() {
			if strings.HasPrefix(ad.String(), event) {
				return true, nil
			}

			log.Printf("ignoring ACPI event: %q", ad.String())
		}
	}

	return false, result.ErrorOrNil()
}
