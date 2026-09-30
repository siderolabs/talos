// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

// Package terminal provides shared terminal input, output, and raw-mode mechanics.
package terminal

import (
	"context"
	"errors"
	"fmt"
	"io"
)

// Input is an exclusively owned, pollable terminal input source.
type Input interface {
	io.Reader
	Fd() uintptr
}

// ReadInput reads bounded chunks until input, consume, or ctx terminates.
// Each non-empty data slice is owned by consume and is never reused by ReadInput.
// Cancellation is checked while waiting for descriptor readiness; Read itself must
// return after readiness. ReadInput does not start background workers or close input.
func ReadInput(ctx context.Context, input Input, chunkSize int, consume func([]byte) error) error {
	if err := validateReadInput(input, chunkSize, consume); err != nil {
		return err
	}

	for {
		if err := waitReadable(ctx, input.Fd()); err != nil {
			return err
		}

		data, readErr := readOwnedChunk(input, chunkSize)
		if len(data) > 0 {
			if err := consume(data); err != nil {
				return err
			}
		}

		if readErr != nil {
			return fmt.Errorf("read terminal input: %w", readErr)
		}
	}
}

func validateReadInput(input Input, chunkSize int, consume func([]byte) error) error {
	if input == nil {
		return errors.New("terminal input must not be nil")
	}

	if chunkSize <= 0 {
		return errors.New("terminal input chunk size must be positive")
	}

	if consume == nil {
		return errors.New("terminal input consumer must not be nil")
	}

	return nil
}

func readOwnedChunk(input Input, chunkSize int) ([]byte, error) {
	buffer := make([]byte, chunkSize)

	n, err := input.Read(buffer)
	if n < 0 || n > len(buffer) {
		return nil, fmt.Errorf("read terminal input: invalid byte count %d", n)
	}

	return buffer[:n:n], err
}
