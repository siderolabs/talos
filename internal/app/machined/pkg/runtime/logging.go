// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import "io"

// LoggingManager provides unified interface to publish and consume logs.
type LoggingManager interface {
	ServiceLog(service string) LogHandler
}

// LogOptions for LogHandler.Reader.
type LogOptions struct {
	Follow    bool
	TailLines *int
}

// LogOption provides functional options for LogHandler.Reader.
type LogOption func(*LogOptions) error

// WithFollow enables follow mode for the logs.
func WithFollow() LogOption {
	return func(o *LogOptions) error {
		o.Follow = true

		return nil
	}
}

// WithTailLines starts log reading from lines from the tail of the log.
func WithTailLines(lines int) LogOption {
	return func(o *LogOptions) error {
		o.TailLines = &lines

		return nil
	}
}

// LogHandler provides interface to access particular log file.
type LogHandler interface {
	Writer() (io.WriteCloser, error)
	Reader(opt ...LogOption) (io.ReadCloser, error)
}
