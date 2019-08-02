/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package nic

import (
	"errors"

	"github.com/hashicorp/go-multierror"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/address"
)

// TODO: Probably should put together some map here
// to map number with string()
const (
	Bond = iota
	Single

	// https://tools.ietf.org/html/rfc791
	MinimumMTU = 68
	MaximumMTU = 65536
)

// NetworkInterface provides an abstract configuration representation for a
// network interface
type NetworkInterface struct {
	Name          string
	Type          int
	MTU           uint32
	Index         uint32
	SubInterfaces []string
	AddressMethod []address.Addressing
}

// Create returns a NetworkInterface with all of the given setter options
// applied
func Create(setters ...Option) (*NetworkInterface, error) {

	// Default interface setup
	iface := defaultOptions()

	// Configure interface with any specified options
	var result *multierror.Error
	for _, setter := range setters {
		result = multierror.Append(setter(iface))
	}

	// TODO: May need to look at switching this around to filter by Interface.HardwareAddr
	// Ensure we have an interface name defined
	if iface.Name == "" {
		result = multierror.Append(errors.New("interface must have a name"))
	}

	// If no addressing methods have been configured, default to DHCP
	// TODO: do we want this behavior or to be explicit with userdata
	// so we dont configure every interface be default?
	if len(iface.AddressMethod) == 0 {
		iface.AddressMethod = append(iface.AddressMethod, &address.DHCP{})
	}

	return iface, result.ErrorOrNil()
}
