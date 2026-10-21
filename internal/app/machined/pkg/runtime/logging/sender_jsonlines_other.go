// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

package logging

import "time"

// setTCPUserTimeout is a no-op: TCP_USER_TIMEOUT is Linux-specific.
func setTCPUserTimeout(uintptr, time.Duration) error {
	return nil
}
