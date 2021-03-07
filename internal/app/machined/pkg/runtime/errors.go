// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"errors"
	"fmt"
)

var (
	// ErrLocked indicates that the sequencer is currently locked, and processing
	// another sequence.
	ErrLocked = errors.New("locked")

	// ErrInvalidSequenceData indicates that the sequencer got data the wrong
	// data type for a sequence.
	ErrInvalidSequenceData = errors.New("invalid sequence data")

	// ErrUndefinedRuntime indicates that the sequencer's runtime is not defined.
	ErrUndefinedRuntime = errors.New("undefined runtime")
)

// RebootError encapsulates unix.Reboot() cmd argument.
type RebootError struct {
	Cmd int
}

func (e RebootError) Error() string {
	return fmt.Sprintf("unix.Reboot(%x)", e.Cmd)
}

// IsRebootError checks whether given error is RebootError.
func IsRebootError(err error) bool {
	var rebootErr RebootError

	return errors.As(err, &rebootErr)
}
