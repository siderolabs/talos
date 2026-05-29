// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package multiplex implements client-side multiplexing helpers.
package multiplex

// Response represents a multiplexed response from a specific node.
type Response[ResponseT any] struct {
	Node    string
	Payload ResponseT
	Err     error
}
