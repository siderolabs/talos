// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:revive
package utils

import (
	"os"
	"strconv"
	"time"
)

// SourceDateEpoch returns parsed value of SOURCE_DATE_EPOCH.
func SourceDateEpoch() (int64, bool, error) {
	epoch, ok := os.LookupEnv("SOURCE_DATE_EPOCH")
	if !ok {
		return 0, false, nil
	}

	epochInt, err := strconv.ParseInt(epoch, 10, 64)
	if err != nil {
		return 0, false, err
	}

	return epochInt, true, nil
}

// SourceDateEpochTime returns the build timestamp as a time.Time in UTC.
//
// If SOURCE_DATE_EPOCH is set, it is used, otherwise the current time is returned.
//
// The returned time is always in UTC, so that it serializes identically regardless
// of the local timezone of the build host (e.g. in OCI image config 'created' field).
func SourceDateEpochTime() (time.Time, error) {
	epoch, ok, err := SourceDateEpoch()
	if err != nil {
		return time.Time{}, err
	}

	if !ok {
		return time.Now().UTC(), nil
	}

	return time.Unix(epoch, 0).UTC(), nil
}
