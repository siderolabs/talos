// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package reaper

import (
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type zombieHunter struct {
	mu sync.Mutex

	running   bool
	listeners map[chan<- ProcessInfo]struct{}
	ready     chan struct{}
	shutdown  chan struct{}
}

func (zh *zombieHunter) Run() {
	zh.mu.Lock()
	defer zh.mu.Unlock()

	if zh.running {
		panic("zombie hunter is already running")
	}

	zh.running = true

	zh.ready = make(chan struct{})
	zh.shutdown = make(chan struct{})
	zh.listeners = make(map[chan<- ProcessInfo]struct{})

	go zh.run()

	<-zh.ready
}

func (zh *zombieHunter) Shutdown() {
	zh.mu.Lock()
	running := zh.running
	zh.mu.Unlock()

	if !running {
		return
	}

	zh.shutdown <- struct{}{}
	<-zh.shutdown
}

func (zh *zombieHunter) Notify(ch chan<- ProcessInfo) bool {
	zh.mu.Lock()
	defer zh.mu.Unlock()

	if !zh.running {
		return false
	}

	zh.listeners[ch] = struct{}{}

	return true
}

func (zh *zombieHunter) Stop(ch chan<- ProcessInfo) {
	zh.mu.Lock()
	defer zh.mu.Unlock()

	delete(zh.listeners, ch)
}

func (zh *zombieHunter) run() {
	sigCh := make(chan os.Signal, 1)

	signal.Notify(sigCh, syscall.SIGCHLD)
	defer signal.Stop(sigCh)

	zh.ready <- struct{}{}

	for {
		// wait for SIGCHLD
		select {
		case <-sigCh:
		case <-zh.shutdown:
			zh.mu.Lock()
			zh.running = false
			zh.mu.Unlock()

			zh.shutdown <- struct{}{}

			return
		}

		// reap all the zombies
		zh.reapLoop()
	}
}

// reapLoop processes all the known zombies at the moment.
func (zh *zombieHunter) reapLoop() {
	for {
		var (
			wstatus syscall.WaitStatus
			pid     int
			err     error
		)

		for {
			// retry EINTR on wait4()
			pid, err = syscall.Wait4(-1, &wstatus, syscall.WNOHANG, nil)
			if err != syscall.EINTR {
				break
			}
		}

		if err == syscall.ECHILD || pid == 0 {
			// no more zombies
			return
		}

		if err != nil {
			log.Printf("zombie reaper error in wait4: %s", err)

			return
		}

		zh.send(pid, wstatus)
	}
}

// send notification about reaped zombie to all listeners.
func (zh *zombieHunter) send(pid int, wstatus syscall.WaitStatus) {
	zh.mu.Lock()

	listeners := make([]chan<- ProcessInfo, 0, len(zh.listeners))
	for ch := range zh.listeners {
		listeners = append(listeners, ch)
	}

	zh.mu.Unlock()

	notification := ProcessInfo{
		Pid:    pid,
		Status: wstatus,
	}

	for _, listener := range listeners {
		select {
		case listener <- notification:
		default: // drop notifications if listener is not keeping up
		}
	}
}
