// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"sync"

	"github.com/talos-systems/talos/pkg/provision/providers/vm"
)

// PowerState is current VM power state.
type PowerState string

// Virtual machine power state.
const (
	PoweredOn  PowerState = "on"
	PoweredOff PowerState = "off"
)

// VMCommand is a translated VM command.
type VMCommand string

// Virtual machine commands.
const (
	VMCommandStart VMCommand = "start"
	VMCommandStop  VMCommand = "stop"
)

// Controller supports IPMI-like machine control.
type Controller struct {
	mu    sync.Mutex
	state PowerState

	forcePXEBoot bool

	commandsCh chan VMCommand
}

// NewController initializes controller in "powered on" state.
func NewController() *Controller {
	return &Controller{
		state:      PoweredOn,
		commandsCh: make(chan VMCommand),
	}
}

// PowerOn implements vm.Controller interface.
func (c *Controller) PowerOn() error {
	c.mu.Lock()

	if c.state == PoweredOn {
		c.mu.Unlock()

		return nil
	}

	c.state = PoweredOn
	c.mu.Unlock()

	c.commandsCh <- VMCommandStart

	return nil
}

// PowerOff implements vm.Controller interface.
func (c *Controller) PowerOff() error {
	c.mu.Lock()

	if c.state == PoweredOff {
		c.mu.Unlock()

		return nil
	}

	c.state = PoweredOff
	c.mu.Unlock()

	c.commandsCh <- VMCommandStop

	return nil
}

// Reboot implements vm.Controller interface.
func (c *Controller) Reboot() error {
	c.mu.Lock()

	if c.state == PoweredOff {
		c.mu.Unlock()

		return nil
	}

	c.mu.Unlock()

	c.commandsCh <- VMCommandStop

	return nil
}

// PXEBootOnce implements vm.Controller interface.
func (c *Controller) PXEBootOnce() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.forcePXEBoot = true

	return nil
}

// Status implements vm.Controller interface.
func (c *Controller) Status() vm.Status {
	return vm.Status{
		PoweredOn: c.PowerState() == PoweredOn,
	}
}

// PowerState returns current power state.
func (c *Controller) PowerState() PowerState {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.state
}

// ForcePXEBoot returns whether next boot should be PXE boot.
func (c *Controller) ForcePXEBoot() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.forcePXEBoot {
		c.forcePXEBoot = false

		return true
	}

	return false
}

// CommandsCh returns channel with commands.
func (c *Controller) CommandsCh() <-chan VMCommand {
	return c.commandsCh
}
