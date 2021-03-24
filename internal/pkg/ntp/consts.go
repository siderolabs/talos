// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import "time"

const (
	// MaxAllowablePoll is the 'recommended' interval for querying a time server.
	MaxAllowablePoll = 1024 * time.Second
	// MinAllowablePoll is the minimum time allowed for a client to query a time server.
	MinAllowablePoll = 4 * time.Second
	// AdjustTimeLimit is a maximum time drift to compensate via adjtimex().
	AdjustTimeLimit = 128 * time.Millisecond
	// EpochLimit is a minimum time difference to signal that change as epoch change.
	EpochLimit = 15 * time.Minute
)
