// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kexec call unix.KexecFileLoad with error handling.
package kexec

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	goruntime "runtime"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/zboot"
)

// Load handles zboot for arm64 and calls unix.KexecFileLoad with error handling and sets the machine state to kexec prepared.
func Load(r runtime.Runtime, kernel *os.File, initrdFD int, cmdline string) error {
	kernelFD := int(kernel.Fd())

	// on arm64 we need to extract the kernel from the zboot image if it's compressed
	if goruntime.GOARCH == "arm64" {
		var (
			fileCloser io.Closer
			extractErr error
		)

		kernelFD, fileCloser, extractErr = zboot.Extract(kernel)
		if extractErr != nil {
			return fmt.Errorf("failed to extract kernel from zboot: %w", extractErr)
		}

		defer func() {
			if fileCloser != nil {
				fileCloser.Close() //nolint:errcheck
			}
		}()
	}

	if err := unix.KexecFileLoad(kernelFD, initrdFD, cmdline, 0); err != nil {
		if reason := loadSkipReason(err); reason != "" {
			log.Print(reason)

			return nil
		}

		return fmt.Errorf("error loading kernel for kexec: %w", err)
	}

	r.State().Machine().KexecPrepared(true)

	return nil
}

// loadSkipReason returns a non-empty reason if the error from kexec_file_load means that kexec can't be used on this machine.
//
// The reboot then continues without kexec, as a normal reboot through the firmware: failing the reboot sequence
// would make machined revert a pending upgrade although the new image was never booted.
func loadSkipReason(err error) string {
	switch {
	case errors.Is(err, unix.ENOSYS):
		return "kexec support is disabled in the kernel"
	case errors.Is(err, unix.EPERM):
		return "kexec support is disabled via sysctl"
	case errors.Is(err, unix.EBUSY):
		return "kexec is busy"
	case errors.Is(err, unix.EADDRNOTAVAIL):
		// the kernel found no free memory region for the new kernel (arm64: "Could not find any suitable kernel location!")
		return "kexec is skipped as there is no free memory region for the kernel"
	default:
		return ""
	}
}
