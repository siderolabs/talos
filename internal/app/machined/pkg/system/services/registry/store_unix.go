// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build darwin || freebsd || openbsd || netbsd || dragonfly

package registry

import (
	"os"
	"syscall"
	"time"
)

func getATime(fi os.FileInfo) time.Time {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		return time.Unix(int64(st.Atimespec.Sec), int64(st.Atimespec.Nsec))
	}

	return fi.ModTime()
}
