// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import "math"

// NfTablesChainPriority wraps nftables.ChainPriority for YAML marshaling.
type NfTablesChainPriority int32

// Constants copied from nftables to provide Stringer interface.
//
//structprotogen:gen_enum
const (
	ChainPriorityFirst           NfTablesChainPriority = math.MinInt32 // first
	ChainPriorityConntrackDefrag NfTablesChainPriority = -400          // conntrack-defrag
	ChainPriorityRaw             NfTablesChainPriority = -300          // raw
	ChainPrioritySELinuxFirst    NfTablesChainPriority = -225          // selinux-first
	ChainPriorityConntrack       NfTablesChainPriority = -200          // conntrack
	ChainPriorityMangle          NfTablesChainPriority = -150          // mangle
	ChainPriorityNATDest         NfTablesChainPriority = -100          // nat-dest
	ChainPriorityFilter          NfTablesChainPriority = 0             // filter
	ChainPrioritySecurity        NfTablesChainPriority = 50            // security
	ChainPriorityNATSource       NfTablesChainPriority = 100           // nat-source
	ChainPrioritySELinuxLast     NfTablesChainPriority = 225           // selinux-last
	ChainPriorityConntrackHelper NfTablesChainPriority = 300           // conntrack-helper
	ChainPriorityLast            NfTablesChainPriority = math.MaxInt32 // last
)
