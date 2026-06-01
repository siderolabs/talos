// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// MatchOperator is a netfilter match operator.
type MatchOperator int

// MatchOperator constants.
//
//structprotogen:gen_enum
const (
	OperatorEqual    MatchOperator = 0 // ==
	OperatorNotEqual MatchOperator = 1 // !=
)
