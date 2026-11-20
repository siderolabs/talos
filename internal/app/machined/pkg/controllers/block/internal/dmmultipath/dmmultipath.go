// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package dmmultipath implements the bits of path management of a device-mapper multipath device
// which the kernel leaves to userspace: telling which paths have failed, and reinstating them.
package dmmultipath

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// ControlPath is the device-mapper control device.
const ControlPath = "/dev/mapper/control"

// statusBufferSize is large enough for the status of a multipath device with hundreds of paths.
const statusBufferSize = 64 * 1024

// PathStates returns the state the kernel keeps for each path of the multipath device with the
// UUID, keyed by the device number of the path ("major:minor"): true for an active path, false for
// a failed one.
func PathStates(uuid string) (map[string]bool, error) {
	buf, err := ioctl(unix.DM_TABLE_STATUS, uuid, statusBufferSize, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to get the status of device-mapper device %s: %w", uuid, err)
	}

	header := (*unix.DmIoctl)(unsafe.Pointer(&buf[0]))

	if header.Flags&unix.DM_BUFFER_FULL_FLAG != 0 {
		return nil, fmt.Errorf("the status of device-mapper device %s exceeds %d bytes", uuid, statusBufferSize)
	}

	if header.Target_count != 1 {
		return nil, fmt.Errorf("device-mapper device %s has %d targets, expected a single multipath target", uuid, header.Target_count)
	}

	spec := (*unix.DmTargetSpec)(unsafe.Pointer(&buf[header.Data_start]))

	if targetType := string(bytes.TrimRight(spec.Target_type[:], "\x00")); targetType != "multipath" {
		return nil, fmt.Errorf("device-mapper device %s is not a multipath device, but %q", uuid, targetType)
	}

	status := buf[int(header.Data_start)+unix.SizeofDmTargetSpec:]
	status, _, _ = bytes.Cut(status, []byte{0})

	return ParsePathStates(string(status)), nil
}

// ParsePathStates extracts the state of each path from the status of a multipath target.
//
// Each path is reported as "<major>:<minor> <A|F> <fail count>", surrounded by fields of the
// path groups and the path selectors which vary in number, so the paths are picked out by their shape.
func ParsePathStates(status string) map[string]bool {
	states := map[string]bool{}

	fields := strings.Fields(status)

	for i := 0; i+1 < len(fields); i++ {
		if !isDevNo(fields[i]) {
			continue
		}

		switch fields[i+1] {
		case "A":
			states[fields[i]] = true
		case "F":
			states[fields[i]] = false
		}
	}

	return states
}

func isDevNo(s string) bool {
	major, minor, ok := strings.Cut(s, ":")

	return ok && isDigits(major) && isDigits(minor)
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}

	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}

	return true
}

// ReinstatePath brings back a failed path ("major:minor") of the multipath device with the UUID.
func ReinstatePath(uuid, path string) error {
	return message(uuid, "reinstate_path "+path)
}

// FailPath fails a path ("major:minor") of the multipath device with the UUID, so that no I/O is
// sent to it until it is reinstated.
func FailPath(uuid, path string) error {
	return message(uuid, "fail_path "+path)
}

// message sends a message to the target of the device-mapper device with the UUID.
func message(uuid, msg string) error {
	// struct dm_target_msg: the sector the message is for (any, as there is a single target), then
	// the message as a NUL-terminated string
	payload := make([]byte, 8, 8+len(msg)+1)
	payload = append(payload, msg+"\x00"...)

	if _, err := ioctl(unix.DM_TARGET_MSG, uuid, unix.SizeofDmIoctl+len(payload), payload); err != nil {
		return fmt.Errorf("failed to send %q to device-mapper device %s: %w", msg, uuid, err)
	}

	return nil
}

// ioctl runs a device-mapper ioctl on the device with the UUID, with the payload following the header.
func ioctl(request uintptr, uuid string, size int, payload []byte) ([]byte, error) {
	if len(uuid) >= len(unix.DmIoctl{}.Uuid) {
		return nil, fmt.Errorf("device-mapper UUID %q is too long", uuid)
	}

	// the buffer is 8-byte aligned, as the kernel expects of struct dm_ioctl
	words := make([]uint64, (size+7)/8)
	buf := unsafe.Slice((*byte)(unsafe.Pointer(&words[0])), len(words)*8)

	header := (*unix.DmIoctl)(unsafe.Pointer(&buf[0]))
	header.Version = [3]uint32{unix.DM_VERSION_MAJOR, 0, 0}
	header.Data_size = uint32(len(buf))
	header.Data_start = unix.SizeofDmIoctl
	copy(header.Uuid[:], uuid)

	copy(buf[unix.SizeofDmIoctl:], payload)

	control, err := os.OpenFile(ControlPath, os.O_RDWR|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}

	defer control.Close() //nolint:errcheck

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, control.Fd(), request, uintptr(unsafe.Pointer(&buf[0]))); errno != 0 {
		return nil, errno
	}

	return buf, nil
}

// directIOSize is read from a path to check it: the largest logical block size of a disk, so that
// the read is aligned for any disk.
const directIOSize = 4096

// CheckPath reads the first block of the path device, bypassing the page cache, to tell whether the
// path can serve I/O, which is what the "directio" checker of multipath-tools does.
//
// A read from a path which can't be reached might only fail once the SCSI layer gives up on it, so
// the check gives up after the timeout, leaving the read to finish in the background.
func CheckPath(ctx context.Context, devPath string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	errCh := make(chan error, 1)

	go func() {
		errCh <- readDirect(devPath)
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return fmt.Errorf("reading %s: %w", devPath, ctx.Err())
	}
}

func readDirect(devPath string) error {
	f, err := os.OpenFile(devPath, os.O_RDONLY|unix.O_DIRECT|unix.O_CLOEXEC, 0)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	// direct I/O needs a buffer aligned to the logical block size, which an anonymous mapping
	// (aligned to a page) always is
	buf, err := unix.Mmap(-1, 0, directIOSize, unix.PROT_READ|unix.PROT_WRITE, unix.MAP_PRIVATE|unix.MAP_ANONYMOUS)
	if err != nil {
		return err
	}

	defer unix.Munmap(buf) //nolint:errcheck

	_, err = f.ReadAt(buf, 0)

	return err
}
