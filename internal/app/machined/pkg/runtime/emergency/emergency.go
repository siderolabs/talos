// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package emergency provides values to handle emergency (panic/unrecoverable error) handling for machined.
package emergency

import (
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// RebootCmd is a command to reboot the system after an unrecoverable error.
var RebootCmd atomic.Int64

func init() {
	RebootCmd.Store(unix.LINUX_REBOOT_CMD_RESTART)
}
