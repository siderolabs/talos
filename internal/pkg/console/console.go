// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package console contains console-related functionality.
package console

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	// vtActivate activates the specified virtual terminal.
	// See VT_ACTIVATE:
	// https://man7.org/linux/man-pages/man2/ioctl_console.2.html
	// https://github.com/torvalds/linux/blob/v6.2/include/uapi/linux/vt.h#L42
	vtActivate uintptr = 0x5606

	// tioclSetKmsgRedirect redirects kernel messages to the specified tty.
	// See TIOCL_SETKMSGREDIRECT:
	// https://github.com/torvalds/linux/blob/v6.2/include/uapi/linux/tiocl.h#L33
	// https://github.com/torvalds/linux/blob/v6.2/drivers/tty/vt/vt.c#L3242
	tioclSetKmsgRedirect byte = 11
)

// Switch switches the active console to the specified tty.
func Switch(ttyNumber int) error {
	// redirect the kernel logs to their own TTY instead of the currently used one,
	// so that other TTYs (e.g., dashboard on tty2) do not get flooded with kernel logs
	if err := redirectKernelLogs(constants.KernelLogsTTY); err != nil {
		return err
	}

	// we need a valid fd to any tty because ioctl requires it
	tty0, err := os.OpenFile("/dev/tty0", os.O_RDWR, 0)
	if err != nil {
		return err
	}

	defer tty0.Close() //nolint:errcheck

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, tty0.Fd(), vtActivate, uintptr(ttyNumber)); errno != 0 {
		return fmt.Errorf("failed to activate console: %w", errno)
	}

	return nil
}

// redirectKernelLogs redirects kernel logs to the specified tty.
func redirectKernelLogs(ttyNumber int) error {
	tty, err := os.OpenFile(fmt.Sprintf("/dev/tty%d", ttyNumber), os.O_RDWR, 0)
	if err != nil {
		return err
	}

	args := [2]byte{tioclSetKmsgRedirect, byte(ttyNumber)}

	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, tty.Fd(), syscall.TIOCLINUX, uintptr(unsafe.Pointer(&args))); errno != 0 {
		return fmt.Errorf("failed to set redirect for kmsg: %w", errno)
	}

	return tty.Close()
}
