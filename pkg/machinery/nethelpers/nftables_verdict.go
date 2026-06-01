// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// NfTablesVerdict wraps nftables.Verdict for YAML marshaling.
type NfTablesVerdict int64

// Constants copied from nftables to provide Stringer interface.
//
//structprotogen:gen_enum
const (
	VerdictDrop   NfTablesVerdict = 0 // drop
	VerdictAccept NfTablesVerdict = 1 // accept
)
