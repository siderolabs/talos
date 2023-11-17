// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// NfTablesChainType defines what this chain will be used for. See also
// https://wiki.nftables.org/wiki-nftables/index.php/Configuring_chains#Base_chain_types
type NfTablesChainType = string

// Possible ChainType values.
const (
	ChainTypeFilter NfTablesChainType = "filter"
	ChainTypeRoute  NfTablesChainType = "route"
	ChainTypeNAT    NfTablesChainType = "nat"
)
