// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemuimg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
)

// ValidationError identifies a permanent unsupported input or request.
type ValidationError struct{ Err error }

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

// ToolUnavailableError identifies a missing or unexecutable required tool.
type ToolUnavailableError struct{ Err error }

func (e *ToolUnavailableError) Error() string { return e.Err.Error() }
func (e *ToolUnavailableError) Unwrap() error { return e.Err }

// IsTerminal classifies only permanent validation and tool availability failures.
// Child exits (including ENOSPC/EIO), cancellation and runtime failures can retry.
func IsTerminal(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}

	var (
		validation  *ValidationError
		unavailable *ToolUnavailableError
	)

	return errors.As(err, &validation) || errors.As(err, &unavailable)
}

func invalid(format string, args ...any) error {
	return &ValidationError{Err: fmt.Errorf(format, args...)}
}

// ExecError retains bounded subprocess diagnostics. Exit codes are retryable.
type ExecError struct {
	ExitCode int
	Stderr   []byte
}

func (e *ExecError) Error() string {
	message := fmt.Sprintf("qemu-img failed with exit code %d", e.ExitCode)
	if diagnostic := sanitizeDiagnostic(e.Stderr); diagnostic != "" {
		message += ": " + diagnostic
	}

	return message
}

func sanitizeDiagnostic(data []byte) string {
	const limit = 1024

	var result strings.Builder

	for _, r := range string(data) {
		if unicode.IsControl(r) || unicode.IsSpace(r) || unicode.In(r, unicode.Cf) {
			r = ' '
		}

		if result.Len()+len(string(r)) > limit {
			result.WriteString("...")

			break
		}

		result.WriteRune(r)
	}

	return strings.TrimSpace(result.String())
}
