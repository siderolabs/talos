// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "golang.org/x/sys/unix"

//go:generate stringer -type=RouteProtocol -linecomment -output routeprotocol_string_linux.go

// RouteProtocol is a routing protocol.
type RouteProtocol uint8

// MarshalYAML implements yaml.Marshaler.
func (rp RouteProtocol) MarshalYAML() (interface{}, error) {
	return rp.String(), nil
}

// RouteType constants.
const (
	ProtocolUnspec   RouteProtocol = unix.RTPROT_UNSPEC   // unspec
	ProtocolRedirect RouteProtocol = unix.RTPROT_REDIRECT // redirect
	ProtocolKernel   RouteProtocol = unix.RTPROT_KERNEL   // kernel
	ProtocolBoot     RouteProtocol = unix.RTPROT_BOOT     // boot
	ProtocolStatic   RouteProtocol = unix.RTPROT_STATIC   // static
)
