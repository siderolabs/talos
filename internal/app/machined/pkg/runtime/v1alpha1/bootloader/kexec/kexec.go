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
		switch {
		case errors.Is(err, unix.ENOSYS):
			log.Printf("kexec support is disabled in the kernel")

			return nil
		case errors.Is(err, unix.EPERM):
			log.Printf("kexec support is disabled via sysctl")

			return nil
		case errors.Is(err, unix.EBUSY):
			log.Printf("kexec is busy")

			return nil
		default:
			return fmt.Errorf("error loading kernel for kexec: %w", err)
		}
	}

	r.State().Machine().KexecPrepared(true)

	return nil
}
