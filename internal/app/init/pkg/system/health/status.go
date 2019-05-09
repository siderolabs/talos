/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package health

import (
	"sync"
	"time"
)

// Status of the healthcheck
type Status struct {
	Healthy     *bool
	LastChange  time.Time
	LastMessage string
}

// State provides proper locking around health state
type State struct {
	sync.Mutex

	status Status
}

// Update health status (locked)
func (state *State) Update(healthy bool, message string) {
	state.Lock()
	defer state.Unlock()

	state.status.LastMessage = message
	if state.status.Healthy == nil || *state.status.Healthy != healthy {
		state.status.Healthy = &healthy
		state.status.LastChange = time.Now()
	}
}

// Init health status (locked)
func (state *State) Init() {
	state.Lock()
	defer state.Unlock()

	state.status.LastMessage = "Unknown"
	state.status.LastChange = time.Now()
	state.status.Healthy = nil
}

// Get returns health status (locked)
func (state *State) Get() Status {
	state.Lock()
	defer state.Unlock()

	return state.status
}
