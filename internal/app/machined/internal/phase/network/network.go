/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package network

import (
	"log"

	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/nic"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// UserDefinedNetwork represents the UserDefinedNetwork task.
type UserDefinedNetwork struct{}

// NewUserDefinedNetworkTask initializes and returns an UserDefinedNetwork task.
func NewUserDefinedNetworkTask() phase.Task {
	return &UserDefinedNetwork{}
}

// RuntimeFunc returns the runtime function.
func (task *UserDefinedNetwork) RuntimeFunc(mode runtime.Mode) phase.RuntimeFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

// nolint: gocyclo
func (task *UserDefinedNetwork) runtime(args *phase.RuntimeArgs) (err error) {
	nwd, err := networkd.New()
	if err != nil {
		return err
	}

	// Convert links to nic
	log.Println("discovering local network interfaces")
	netconf, err := nwd.Discover()
	if err != nil {
		return err
	}

	// Configure specified interface
	netIfaces := make([]*nic.NetworkInterface, 0, len(netconf))
	var iface *nic.NetworkInterface
	for link, opts := range netconf {
		iface, err = nic.Create(link, opts...)
		if err != nil {
			return err
		}

		if iface.IsIgnored() {
			continue
		}

		netIfaces = append(netIfaces, iface)
	}

	// kick off the addressing mechanism
	// Add any necessary routes
	if err = nwd.Configure(netIfaces...); err != nil {
		return err
	}

	// This next chunk is around saving off the hostname if necessary
	hostname := nwd.Hostname(netIfaces...)
	if hostname == "" {
		return nil
	}

	return unix.Sethostname([]byte(hostname))
}
