// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package probe implements repeated health check probes for DNS upstreams.
package probe

import (
	"sync"
	"time"
)

// defaultInterval is the interval between the checks unless set with [Probe.Start].
const defaultInterval = 500 * time.Millisecond

type state int

const (
	idle state = iota
	active
	stopped
)

// Probe runs a health check repeatedly, until it succeeds.
//
// At most one health check loop runs at a time.
type Probe struct {
	mu       sync.Mutex
	state    state
	interval time.Duration
}

// Do starts running the check in the background every interval until it returns nil.
//
// If the check is already running, or the probe is stopped, Do is a no-op.
func (p *Probe) Do(check func() error) {
	p.mu.Lock()

	if p.state != idle {
		p.mu.Unlock()

		return
	}

	p.state = active

	interval := p.interval
	if interval <= 0 {
		interval = defaultInterval
	}

	p.mu.Unlock()

	go func() {
		for check() != nil {
			time.Sleep(interval)

			p.mu.Lock()
			stop := p.state == stopped
			p.mu.Unlock()

			if stop {
				return
			}
		}

		p.mu.Lock()

		if p.state == active {
			p.state = idle
		}

		p.mu.Unlock()
	}()
}

// Start sets the interval between the checks.
func (p *Probe) Start(interval time.Duration) {
	p.mu.Lock()
	p.interval = interval
	p.mu.Unlock()
}

// Stop stops the running check loop (after the check in progress finishes), and prevents new ones.
func (p *Probe) Stop() {
	p.mu.Lock()
	p.state = stopped
	p.mu.Unlock()
}
