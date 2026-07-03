// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

package miniprocfs

// ReadExeIdentity is not supported on non-Linux platforms.
func ReadExeIdentity(int32) (dev, ino uint64, ok bool) {
	return 0, 0, false
}
