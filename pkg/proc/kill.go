// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package proc

import (
	"log"
	"syscall"
	"time"
)

// KillAll kills all processes gracefully then forcefully.
func KillAll() error {
	timer := time.NewTicker(time.Second)
	defer timer.Stop()

	// timeouts in seconds
	const (
		killTimeout     = 15
		gracefulTimeout = 10
	)

	for i := 0; i < killTimeout; i++ {
		pids, err := List()
		if err != nil {
			return err
		}

		if len(pids) == 0 {
			break
		}

		switch i {
		case 0:
			killAll(pids, syscall.SIGTERM)
		case gracefulTimeout:
			killAll(pids, syscall.SIGKILL)
		case killTimeout - 1:
			log.Printf("leaving with %d processes pending", len(pids))
		default:
			log.Printf("waiting for %d processes to terminate", len(pids))
		}

		<-timer.C
	}

	return nil
}

func killAll(pids []int, signal syscall.Signal) {
	for _, pid := range pids {
		syscall.Kill(pid, signal) //nolint: errcheck
	}

	log.Printf("killed %d procs with %s", len(pids), signal)
}
