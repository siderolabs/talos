/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package networkd

import (
	"sync"
)

type singleton struct {
	// State of running services by ID
	state networkd

	mu sync.Mutex
}

var instance *singleton
var once sync.Once

// Services returns the instance of the system services API.
func Conn() *singleton {
	once.Do(func() {
		nwd, err = New()

		instance = &singleton{
			state: networkd,
		}
	})
	return instance
}

func (c *Conn) List() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	ifnames := make([]string, 0, len(c.state.Interfaces))
	for _, v := range c.state.Interfaces {
		ifnames = append(ifnames, v.Name)
	}

	return ifnames
}

func (c *Conn) Describe(string netif) {}
