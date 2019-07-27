/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package api

import (
	"log"
	"time"

	"golang.org/x/sys/unix"
)

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

func reboot() {
	// See http://man7.org/linux/man-pages/man2/reboot.2.html.
	sync()

	// nolint: errcheck
	unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART)

	select {}
}
