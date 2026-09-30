// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package terminal

import (
	"fmt"
	"io"
)

// WriteOutput writes one terminal output frame and rejects partial delivery.
// A generic io.Writer cannot be canceled; callers must provide a writer whose
// Write eventually returns when bounded shutdown is required.
func WriteOutput(output io.Writer, data []byte) error {
	if output == nil {
		return fmt.Errorf("terminal output must not be nil")
	}

	written, err := output.Write(data)
	if err != nil {
		return fmt.Errorf("write terminal output: %w", err)
	}

	if written != len(data) {
		return fmt.Errorf("write terminal output: %w", io.ErrShortWrite)
	}

	return nil
}
