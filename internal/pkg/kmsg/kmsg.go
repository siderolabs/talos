// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kmsg provides access to kernel log.
//
package kmsg

import (
	"fmt"
	"io"
	"log"
	"os"

	"golang.org/x/sys/unix"
)

// SetupLogger configures the logger to write to the kernel ring buffer via
// /dev/kmsg.
//
// If logger is nil, default `log` logger is redirectred.
//
// If extraWriter is not nil, logs will be copied to it as well.
func SetupLogger(logger *log.Logger, prefix string, extraWriter io.Writer) error {
	kmsg, err := os.OpenFile("/dev/kmsg", os.O_RDWR|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOCTTY, 0o666)
	if err != nil {
		return fmt.Errorf("failed to open /dev/kmsg: %w", err)
	}

	var writer io.Writer = &Writer{KmsgWriter: kmsg}

	if extraWriter != nil {
		writer = io.MultiWriter(writer, extraWriter)
	}

	if logger != nil {
		logger.SetOutput(writer)
		logger.SetPrefix(prefix + " ")
		logger.SetFlags(0)
	} else {
		log.SetOutput(writer)
		log.SetPrefix(prefix + " ")
		log.SetFlags(0)
	}

	return nil
}
