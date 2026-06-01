// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// NfTablesChainHook wraps nftables.ChainHook for YAML marshaling.
type NfTablesChainHook uint32

// Constants copied from nftables to provide Stringer interface.
//
//structprotogen:gen_enum
const (
	ChainHookPrerouting  NfTablesChainHook = 0 // prerouting
	ChainHookInput       NfTablesChainHook = 1 // input
	ChainHookForward     NfTablesChainHook = 2 // forward
	ChainHookOutput      NfTablesChainHook = 3 // output
	ChainHookPostrouting NfTablesChainHook = 4 // postrouting
)
