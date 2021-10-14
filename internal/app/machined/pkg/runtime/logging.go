// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"io"
	"time"

	"go.uber.org/zap/zapcore"
)

// LoggingManager provides unified interface to publish and consume logs.
type LoggingManager interface {
	// ServiceLog privides a log handler for a given service (that may not exist).
	ServiceLog(service string) LogHandler

	// SetSender sets the log sender for all derived log handlers.
	SetSender(sender LogSender)
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

// LogHandler provides interface to access particular log source.
type LogHandler interface {
	Writer() (io.WriteCloser, error)
	Reader(opt ...LogOption) (io.ReadCloser, error)
}

// LogEvent represents a log message to be send.
type LogEvent struct {
	Msg    string
	Time   time.Time
	Level  zapcore.Level
	Fields map[string]interface{}
}

// ErrDontRetry indicates that log event should not be resent.
var ErrDontRetry = fmt.Errorf("don't retry")

// LogSender provides common interface for log senders.
type LogSender interface {
	// Send tries to send the log event once, exiting on success, error, or context cancelation.
	// Returned error is nil on success, non-nil otherwise.
	// As a special case, Send can return (possibly wrapped) ErrDontRetry if the log event should not be resent
	// (if it is invalid, if it was sent partially, etc).
	Send(ctx context.Context, e *LogEvent) error
}
