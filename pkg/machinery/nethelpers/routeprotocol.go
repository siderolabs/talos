// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

//go:generate stringer -type=RouteProtocol -linecomment

// RouteProtocol is a routing protocol.
type RouteProtocol uint8

// MarshalYAML implements yaml.Marshaler.
func (rp RouteProtocol) MarshalYAML() (interface{}, error) {
	return rp.String(), nil
}

// RouteType constants.
const (
	ProtocolUnspec   RouteProtocol = iota // unspec
	ProtocolRedirect                      // redirect
	ProtocolKernel                        // kernel
	ProtocolBoot                          // boot
	ProtocolStatic                        // static
)
