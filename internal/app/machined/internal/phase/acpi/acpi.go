/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package acpi

import (
	"log"

	"github.com/mdlayher/genetlink"
	"github.com/mdlayher/netlink"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/event"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// Handler represents the ACPI handler task.
type Handler struct{}

// NewHandlerTask initializes and returns a ACPI handler task.
func NewHandlerTask() phase.Task {
	return &Handler{}
}

// TaskFunc returns the runtime function.
func (task *Handler) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.standard
	}
}

func (task *Handler) standard(r runtime.Runtime) (err error) {
	if err := listenForPowerButton(); err != nil {
		log.Printf("WARNING: power off events will be ignored: %+v", err)
	}

	return nil
}

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

	f, err := conn.GetFamily(acpiGenlFamilyName)
	if netlink.IsNotExist(err) {
		// nolint: errcheck
		conn.Close()
		return errors.Wrap(err, acpiGenlFamilyName+" not available")
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
		return err
	}

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
				log.Printf("shutdown via ACPI received")
				event.Bus().Notify(event.Event{Type: event.Shutdown})
				return
			}
		}
	}()

	return nil
}
