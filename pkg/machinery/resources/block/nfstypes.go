// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

// NFSVersionType describes NFS version type.
type NFSVersionType int

// NFS Version types.
//
//structprotogen:gen_enum
const (
	NFSVersionType4_2 NFSVersionType = iota // 4.2
	NFSVersionType4_1                       // 4.1
	NFSVersionType4                         // 4.0
	NFSVersionType3                         // 3
	NFSVersionType2                         // 2
)

// NFSProtocolType describes NFS protocol type.
type NFSProtocolType int

// NFS Protocol types.
//
//structprotogen:gen_enum
const (
	NFSProtocolTypeTCP   NFSProtocolType = iota // tcp
	NFSProtocolTypeTCP6                         // tcp6
	NFSProtocolTypeUDP                          // udp
	NFSProtocolTypeUDP6                         // udp6
	NFSProtocolTypeRDMA                         // rdma
	NFSProtocolTypeRDMA6                        // rdma6
)
