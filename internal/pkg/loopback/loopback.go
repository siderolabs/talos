// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package loopback provides support for disk loopback devices (/dev/loopN).
package loopback

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Copyright (c) 2017, Paul R. Tagliamonte <paultag@gmail.com>

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// syscalls will return an errno type (which implements error) for all calls,
// including success (errno 0). We only care about non-zero errnos.
func errnoIsErr(err error) error {
	if err.(syscall.Errno) != 0 {
		return err
	}

	return nil
}

// Loop given a handle to a Loopback device (such as /dev/loop0), and a handle
// to the image to loop mount (such as a squashfs or ext4fs image), performs
// the required call to loop the image to the provided block device.
func Loop(loopbackDevice, image *os.File) error {
	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		loopbackDevice.Fd(),
		unix.LOOP_SET_FD,
		image.Fd(),
	)

	return errnoIsErr(err)
}

// LoopSetReadWrite clears the read-only flag on the loop devices.
func LoopSetReadWrite(loopbackDevice *os.File) error {
	var status unix.LoopInfo64

	_, _, err := syscall.Syscall(
		syscall.SYS_IOCTL,
		loopbackDevice.Fd(),
		unix.LOOP_GET_STATUS64,
		uintptr(unsafe.Pointer(&status)),
	)

	if e := errnoIsErr(err); e != nil {
		return e
	}

	status.Flags &= ^uint32(unix.LO_FLAGS_READ_ONLY)

	_, _, err = syscall.Syscall(
		syscall.SYS_IOCTL,
		loopbackDevice.Fd(),
		unix.LOOP_SET_STATUS64,
		uintptr(unsafe.Pointer(&status)),
	)

	runtime.KeepAlive(status)

	return errnoIsErr(err)
}

// Unloop given a handle to the Loopback device (such as /dev/loop0), preforms the
// required call to the image to unloop the file.
func Unloop(loopbackDevice *os.File) error {
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, loopbackDevice.Fd(), unix.LOOP_CLR_FD, 0)

	return errnoIsErr(err)
}

// NextLoopDevice gets the next loopback device that isn't used.
//
// Under the hood this will ask  loop-control for the LOOP_CTL_GET_FREE value, and interpolate
// that into the conventional GNU/Linux naming scheme for loopback devices, and os.Open
// that path.
func NextLoopDevice() (*os.File, error) {
	loopInt, err := nextUnallocatedLoop()
	if err != nil {
		return nil, err
	}

	return os.OpenFile(fmt.Sprintf("/dev/loop%d", loopInt), os.O_RDWR, 0)
}

// Return the integer of the next loopback device we can use by calling
// loop-control with the LOOP_CTL_GET_FREE ioctl.
func nextUnallocatedLoop() (int, error) {
	fd, err := os.OpenFile("/dev/loop-control", os.O_RDONLY, 0o644)
	if err != nil {
		return 0, err
	}

	defer fd.Close() //nolint: errcheck

	index, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd.Fd(), unix.LOOP_CTL_GET_FREE, 0)

	return int(index), errnoIsErr(err)
}
