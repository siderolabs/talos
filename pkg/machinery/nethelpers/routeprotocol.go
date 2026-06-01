// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// RouteProtocol is a routing protocol.
type RouteProtocol uint8

// RouteType constants.
//
//structprotogen:gen_enum
const (
	ProtocolUnspec     RouteProtocol = 0   // unspec
	ProtocolRedirect   RouteProtocol = 1   // redirect
	ProtocolKernel     RouteProtocol = 2   // kernel
	ProtocolBoot       RouteProtocol = 3   // boot
	ProtocolStatic     RouteProtocol = 4   // static
	ProtocolRA         RouteProtocol = 9   // ra
	ProtocolMRT        RouteProtocol = 10  // mrt
	ProtocolZebra      RouteProtocol = 11  // zebra
	ProtocolBird       RouteProtocol = 12  // bird
	ProtocolDnrouted   RouteProtocol = 13  // dnrouted
	ProtocolXorp       RouteProtocol = 14  // xorp
	ProtocolNTK        RouteProtocol = 15  // ntk
	ProtocolDHCP       RouteProtocol = 16  // dhcp
	ProtocolMRTD       RouteProtocol = 17  // mrtd
	ProtocolKeepalived RouteProtocol = 18  // keepalived
	ProtocolBabel      RouteProtocol = 42  // babel
	ProtocolOpenr      RouteProtocol = 99  // openr
	ProtocolBGP        RouteProtocol = 186 // bgp
	ProtocolISIS       RouteProtocol = 187 // isis
	ProtocolOSPF       RouteProtocol = 188 // ospf
	ProtocolRIP        RouteProtocol = 189 // rip
	ProtocolEIGRP      RouteProtocol = 192 // eigrp
)
