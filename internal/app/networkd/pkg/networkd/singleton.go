/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"sync"
)

type singleton struct {
	// State of running services by ID
	state *networkd

	mu sync.Mutex
}

var instance *singleton
var once sync.Once

// Services returns the instance of the system services API.
func Instance() *singleton {
	once.Do(func() {
		instance = &singleton{
			state: state,
		}
	})
	return instance
}

func (s *singleton) List() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	ifnames := make([]string, 0, len(s.state.Interfaces()))
	for _, v := range s.state.Interfaces() {
		ifnames = append(ifnames, v.Name)
	}

	return ifnames
}

func (s *singleton) Get(target string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, v := range s.state.Interfaces() {
		if v.Name == target {
			// TODO add more interesting information
			return v.Name
		}
	}

	return ""
}

func (s *singleton) Describe(netif string) {}
