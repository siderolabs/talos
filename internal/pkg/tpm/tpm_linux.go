// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build linux

// Package tpm provides TPM 2.0 support.
package tpm

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/go-tpm/tpm2/transport"
	"golang.org/x/sys/unix"
)

// Device paths of the TPM.
const (
	// RMDevicePath is the TPM 2.0 device behind the kernel resource manager.
	//
	// The kernel creates it only for TPM 2.0 chips.
	RMDevicePath = "/dev/tpmrm0"
	// DevicePath is the raw TPM device, it exists both for TPM 1.2 and TPM 2.0 chips.
	DevicePath = "/dev/tpm0"
)

// ErrNotDevice is returned when the TPM path is not a character device.
var ErrNotDevice = errors.New("TPM path is not a device")

// Open a TPM device.
//
// Tries first /dev/tpmrm0 and then /dev/tpm0.
func Open() (transport.TPMCloser, error) {
	tpm, err := OpenDevice(RMDevicePath)
	if errors.Is(err, os.ErrNotExist) {
		tpm, err = OpenDevice(DevicePath)
	}

	return tpm, err
}

// OpenTPM2 opens the TPM 2.0 device.
//
// The kernel creates /dev/tpmrm0 only for TPM 2.0 chips, so an error which wraps
// [os.ErrNotExist] means there is no TPM 2.0 available: either no TPM at all, or a TPM 1.2.
// Any other error, and any error from the commands sent afterwards, is a real failure of a TPM 2.0 device.
func OpenTPM2() (transport.TPMCloser, error) {
	return OpenDevice(RMDevicePath)
}

// OpenDevice opens the TPM character device at path.
//
// The device is opened in blocking mode, so that each command is executed by the kernel
// synchronously in write(2), and the response is available to read(2) immediately.
//
// The go-tpm linuxtpm transport opens the device with os.OpenFile, which puts it into
// non-blocking mode, so that the first command is executed asynchronously by the kernel,
// and go-tpm waits for the response with poll(2). poll(2) is never restarted after a signal,
// and the Go runtime sends signals (SIGURG) to preempt goroutines, so the command
// fails randomly with EINTR. Go retries EINTR for read(2) and write(2) on the blocking
// [os.File], and go-tpm skips the poll(2) as the transport is not an [os.File].
func OpenDevice(path string) (transport.TPMCloser, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	if fi.Mode()&os.ModeDevice == 0 {
		return nil, fmt.Errorf("%w: %s (%s)", ErrNotDevice, fi.Mode(), path)
	}

	var fd int

	for {
		fd, err = unix.Open(path, unix.O_RDWR|unix.O_CLOEXEC, 0)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}

	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}

	// os.NewFile keeps the blocking mode of the descriptor,
	// and doesn't register it with the Go runtime poller.
	return transport.FromReadWriteCloser(device{os.NewFile(uintptr(fd), path)}), nil
}

// device hides the [os.File] from go-tpm, so that it doesn't poll(2) before reading the response.
type device struct {
	io.ReadWriteCloser
}
