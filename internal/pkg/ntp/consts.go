// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ntp

import "time"

const (
	// MinAllowablePoll is the minimum time allowed for a client to query a time server.
	MinAllowablePoll = 32 * time.Second
	// MaxAllowablePoll is the maximum allowed interval for querying a time server.
	MaxAllowablePoll = 2048 * time.Second
	// RetryPoll is the interval between retries if the error is not Kiss-o-Death.
	RetryPoll = time.Second
	// AdjustTimeLimit is a maximum time drift to compensate via adjtimex().
	//
	// Deltas smaller than AdjustTimeLimit are gradually adjusted (slewed) to approach the network time.
	// Deltas larger than AdjustTimeLimit are set by letting the system time jump.
	AdjustTimeLimit = 400 * time.Millisecond
	// EpochLimit is a minimum time difference to signal that change as epoch change.
	EpochLimit = 15 * time.Minute
	// ExpectedAccuracy is the expected time sync accuracy, used to adjust poll interval.
	ExpectedAccuracy = 200 * time.Millisecond
)
