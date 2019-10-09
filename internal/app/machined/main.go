/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"
	"os"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	machineapi "github.com/talos-systems/talos/api/machine"
	"github.com/talos-systems/talos/internal/app/machined/internal/sequencer"
	"github.com/talos-systems/talos/internal/pkg/event"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/proc/reaper"
	"github.com/talos-systems/talos/pkg/startup"
)

// EventBusObserver is used to subscribe to the event bus.
type EventBusObserver struct {
	*event.Embeddable
}

func recovery() {
	if r := recover(); r != nil {
		log.Printf("%+v\n", r)

		for i := 10; i >= 0; i-- {
			log.Printf("rebooting in %d seconds\n", i)
			time.Sleep(1 * time.Second)
		}

		if unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART) == nil {
			select {}
		}
	}
}

// See http://man7.org/linux/man-pages/man2/reboot.2.html.
func sync() {
	syncdone := make(chan struct{})

	go func() {
		defer close(syncdone)
		unix.Sync()
	}()

	log.Printf("waiting for sync...")

	for i := 29; i >= 0; i-- {
		select {
		case <-syncdone:
			log.Printf("sync done")
			return
		case <-time.After(time.Second):
		}

		if i != 0 {
			log.Printf("waiting %d more seconds for sync to finish", i)
		}
	}

	log.Printf("sync hasn't completed in time, aborting...")
}

// nolint: gocyclo
func main() {
	var err error

	// This is main entrypoint into machined execution, control is passed here
	// from init after switch root.
	//
	// When machined terminates either on normal shutdown (reboot, poweroff), or
	// due to panic, control goes through recovery() and reboot() functions
	// below, which finalize node state - sync buffers, initiate poweroff or
	// reboot. Also on shutdown, other deferred function are called, for example
	// services are gracefully shutdown.

	// On any return from init.main(), initiate host reboot or shutdown handle
	// any panics in the main goroutine, and proceed to reboot() above
	defer recovery()

	// Subscribe to events.
	init := EventBusObserver{&event.Embeddable{}}
	defer close(init.Channel())

	event.Bus().Register(init)

	defer event.Bus().Unregister(init)

	// Initialize process reaper.
	reaper.Run()
	defer reaper.Shutdown()

	// Ensure rng is seeded.
	if err = startup.RandSeed(); err != nil {
		panic(err)
	}

	// Set the PATH env var.
	if err = os.Setenv("PATH", constants.PATH); err != nil {
		panic(errors.New("error setting PATH"))
	}

	// Boot the machine.
	seq := sequencer.New(sequencer.V1Alpha1)

	// Start the boot sequence in a go routine so that we can list for events.
	go func() {
		defer recovery()
		log.Println(err)
		if err := seq.Boot(); err != nil {
			panic(errors.Wrap(err, "boot failed"))
		}
	}()

	rebootFlag := unix.LINUX_REBOOT_CMD_RESTART

	// Wait for an event.

	for {
		switch e := <-init.Channel(); e.Type {
		case event.Shutdown:
			rebootFlag = unix.LINUX_REBOOT_CMD_POWER_OFF
			fallthrough
		case event.Reboot:
			if err := seq.Shutdown(); err != nil {
				panic(errors.Wrap(err, "shutdown failed"))
			}

			sync()

			if unix.Reboot(rebootFlag) == nil {
				select {}
			}
		case event.Upgrade:
			var (
				req *machineapi.UpgradeRequest
				ok  bool
			)

			if req, ok = e.Data.(*machineapi.UpgradeRequest); !ok {
				log.Println("cannot perform upgrade, unexpected data type")
				continue
			}

			if err := seq.Upgrade(req); err != nil {
				panic(errors.Wrap(err, "upgrade failed"))
			}

			event.Bus().Notify(event.Event{Type: event.Reboot})
		}
	}
}
