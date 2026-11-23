// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runner

import "fmt"

// LastLogError is a run failure together with the last log line(s) the process wrote before it.
type LastLogError struct {
	Err     error
	LastLog string
}

// Error implements error, quoting the last log after the failure.
func (e *LastLogError) Error() string {
	return fmt.Sprintf("%v (last log %q)", e.Err, e.LastLog)
}

// Unwrap implements errors.Unwrap.
func (e *LastLogError) Unwrap() error {
	return e.Err
}
