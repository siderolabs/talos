// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

// RoutingRuleAction is a routing rule action.
type RoutingRuleAction uint8

// RoutingRuleAction constants.
//
//structprotogen:gen_enum
const (
	RoutingRuleActionUnspec      RoutingRuleAction = 0 // unspec
	RoutingRuleActionUnicast     RoutingRuleAction = 1 // unicast
	RoutingRuleActionBlackhole   RoutingRuleAction = 6 // blackhole
	RoutingRuleActionUnreachable RoutingRuleAction = 7 // unreachable
	RoutingRuleActionProhibit    RoutingRuleAction = 8 // prohibit
)
