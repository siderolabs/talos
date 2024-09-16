// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package zboot provides a function to extract the kernel from a Zboot image.
package zboot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"

	"github.com/klauspost/compress/zstd"
	"golang.org/x/sys/unix"
)

// fileCloser is an interface that wraps the Close method.
type fileCloser interface {
	Close() error
}

// Extract extracts the kernel from a Zboot image and returns a file descriptor of the extracted kernel.
func Extract(kernel *os.File) (int, fileCloser, error) {
	// https://git.kernel.org/pub/scm/utils/kernel/kexec/kexec-tools.git/tree/include/kexec-pe-zboot.h
	var peZbootheaderData [28]byte

	if _, err := io.ReadFull(kernel, peZbootheaderData[:]); err != nil {
		return 0, nil, err
	}

	// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/drivers/firmware/efi/libstub/zboot-header.S
	// https://git.kernel.org/pub/scm/linux/kernel/git/stable/linux.git/tree/include/linux/pe.h#n42
	if !bytes.Equal(peZbootheaderData[:2], []byte("MZ")) {
		return 0, nil, fmt.Errorf("invalid PE Zboot header")
	}

	// not a Zboot image, return
	if !bytes.Equal(peZbootheaderData[4:8], []byte("zimg")) {
		return int(kernel.Fd()), nil, nil
	}

	payloadOffset := binary.LittleEndian.Uint32(peZbootheaderData[8:12])

	payloadSize := binary.LittleEndian.Uint32(peZbootheaderData[12:16])

	if _, err := kernel.Seek(int64(payloadOffset), io.SeekStart); err != nil {
		return 0, nil, fmt.Errorf("failed to seek to kernel zstd data from vmlinuz.efi: %w", err)
	}

	z, err := zstd.NewReader(io.LimitReader(kernel, int64(payloadSize)))
	if err != nil {
		return 0, nil, fmt.Errorf("failed to create zstd reader: %w", err)
	}

	defer z.Close()

	fd, err := unix.MemfdCreate("vmlinux", 0)
	if err != nil {
		return 0, nil, fmt.Errorf("memfdCreate: %v", err)
	}

	kernelMemfd := os.NewFile(uintptr(fd), "vmlinux")

	if _, err := io.Copy(kernelMemfd, z); err != nil {
		kernelMemfd.Close() //nolint:errcheck

		return 0, nil, fmt.Errorf("failed to copy zstd data to memfd: %w", err)
	}

	if _, err := kernelMemfd.Seek(0, io.SeekStart); err != nil {
		kernelMemfd.Close() //nolint:errcheck

		return 0, nil, fmt.Errorf("failed to seek to start of memfd: %w", err)
	}

	return fd, kernelMemfd, nil
}
