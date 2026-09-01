// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// NFSVersion describes an NFS protocol version.
type NFSVersion int

// NFS protocol versions.
//
//structprotogen:gen_enum
const (
	NFSVersion3       NFSVersion = iota // 3
	NFSVersion4                         // 4
	NFSVersion4Point1                   // 4.1
	NFSVersion4Point2                   // 4.2
)

// NFSLocking controls NFSv3 lock coordination.
type NFSLocking int

// NFS locking modes.
//
//structprotogen:gen_enum
const (
	NFSLockingLocal  NFSLocking = iota // local
	NFSLockingRemote                   // remote
)

// NFSRecovery controls client behavior after an NFS request times out.
type NFSRecovery int

// NFS recovery modes.
//
//structprotogen:gen_enum
const (
	NFSRecoveryHard      NFSRecovery = iota // hard
	NFSRecoverySoft                         // soft
	NFSRecoverySoftError                    // soft-error
)

// NFSSecurity identifies an NFS RPC security flavor.
type NFSSecurity int

// NFS RPC security flavors.
//
//structprotogen:gen_enum
const (
	NFSSecurityNone NFSSecurity = iota // none
	NFSSecuritySys                     // sys
)

// NFSTransport identifies an NFS transport protocol.
type NFSTransport int

// NFS transport protocols.
//
//structprotogen:gen_enum
const (
	NFSTransportTCP  NFSTransport = iota // tcp
	NFSTransportTCP6                     // tcp6
	NFSTransportUDP                      // udp
	NFSTransportUDP6                     // udp6
)

// IsIPv6 reports whether the transport netid asserts the IPv6 address family.
func (i NFSTransport) IsIPv6() bool {
	return i == NFSTransportTCP6 || i == NFSTransportUDP6
}

// IsUDP reports whether the transport netid selects UDP.
func (i NFSTransport) IsUDP() bool {
	return i == NFSTransportUDP || i == NFSTransportUDP6
}
