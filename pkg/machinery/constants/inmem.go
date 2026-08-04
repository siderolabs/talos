// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package constants

import "time"

const (
	// InmemHistoryInitialCapacity is the initial event buffer length for Talos in-memory state.
	//
	// The buffer is shared across all resources.
	InmemHistoryInitialCapacity = 2048
	// InmemHistoryMaxCapacity is the maximum capacity of the event buffer for Talos in-memory state.
	//
	// The max capacity protects against memory exhaustion, a single event slot (if cleared)
	// should use around 100B of memory.
	InmemHistoryMaxCapacity = 65536
	// InmemHistoryGap protects against watching "too far" into the past, preventing buffer overruns.
	InmemHistoryGap = 64
	// InmemCleanupInterval is the interval to invalidate old entries in the event buffer to make them
	// garbage collectable.
	//
	// It never shrinks the buffer, but it releases the memory linked to the event entry - e.g. the actual resource state.
	InmemCleanupInterval = 5 * time.Minute
)
