// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package output streams lifecycle container output.
package output

import (
	"bufio"
	"errors"
	"fmt"
	"io"
)

const maxLineSize = 1024 * 1024

// Stream reads complete lines from r and sends each line via send.
// Lines exceeding maxLineSize are emitted as bounded fragments so the producer
// is always drained and cannot block lifecycle execution on a full pipe.
func Stream(r io.Reader, send func(string) error) error {
	reader := bufio.NewReaderSize(r, maxLineSize)

	var sendErr error

	for {
		message, err := reader.ReadSlice('\n')

		if len(message) > 0 && sendErr == nil {
			if err := send(string(message)); err != nil {
				sendErr = fmt.Errorf("failed to send message: %w", err)
			}
		}

		switch {
		case err == nil:
			continue
		case errors.Is(err, bufio.ErrBufferFull):
			continue
		case errors.Is(err, io.EOF):
			return sendErr
		default:
			return fmt.Errorf("failed to read output: %w", err)
		}
	}
}
