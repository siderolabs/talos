// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"io"
	"log"
	"strings"

	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LogWriter is a wrapper around zap.Logger that implements io.Writer interface.
type LogWriter struct {
	dest  *zap.Logger
	level zapcore.Level
}

// NewWriter creates new log zap log writer.
func NewWriter(l *zap.Logger, level zapcore.Level) io.Writer {
	return &LogWriter{
		dest:  l,
		level: level,
	}
}

// Write implements io.Writer interface.
func (lw *LogWriter) Write(line []byte) (int, error) {
	checked := lw.dest.Check(lw.level, strings.TrimSpace(string(line)))
	if checked == nil {
		return 0, nil
	}

	checked.Write()

	return len(line), nil
}

// logWrapper wraps around standard logger.
type logWrapper struct {
	log *log.Logger
}

// Write implements io.Writer interface.
func (lw *logWrapper) Write(line []byte) (int, error) {
	if lw.log == nil {
		log.Print(string(line))
	} else {
		lw.log.Print(string(line))
	}

	return len(line), nil
}

// StdWriter creates a sync writer that writes all logs to the std logger.
var StdWriter = &logWrapper{nil}

// LogDestination defines logging destination Config.
type LogDestination struct {
	// Level log level.
	level             zapcore.LevelEnabler
	writer            io.Writer
	config            zapcore.EncoderConfig
	suppressThreshold int64
}

// LogDestinationOption defines a log destination encoder config setter.
type LogDestinationOption func(dest *LogDestination)

// WithoutTimestamp disables timestamp.
func WithoutTimestamp() LogDestinationOption {
	return func(dest *LogDestination) {
		dest.config.EncodeTime = nil
	}
}

// WithoutLogLevels disable log level.
func WithoutLogLevels() LogDestinationOption {
	return func(dest *LogDestination) {
		dest.config.EncodeLevel = nil
	}
}

// WithColoredLevels enables log level colored output.
func WithColoredLevels() LogDestinationOption {
	return func(dest *LogDestination) {
		dest.config.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
}

// WithControllerErrorSuppressor creates a new console log controller error suppressor.
func WithControllerErrorSuppressor(threshold int64) LogDestinationOption {
	return func(dest *LogDestination) {
		dest.suppressThreshold = threshold
	}
}

// NewLogDestination creates new log destination.
func NewLogDestination(writer io.Writer, logLevel zapcore.LevelEnabler, options ...LogDestinationOption) *LogDestination {
	config := zap.NewDevelopmentEncoderConfig()
	config.ConsoleSeparator = " "
	config.StacktraceKey = "error"

	dest := &LogDestination{
		level:  logLevel,
		config: config,
		writer: writer,
	}

	for _, option := range options {
		option(dest)
	}

	return dest
}

// Wrap is a simple helper to wrap io.Writer with default arguments.
func Wrap(writer io.Writer) *zap.Logger {
	return ZapLogger(
		NewLogDestination(writer, zapcore.DebugLevel),
	)
}

// ZapLogger creates new default Zap Logger.
func ZapLogger(dests ...*LogDestination) *zap.Logger {
	if len(dests) == 0 {
		panic("at least one writer must be defined")
	}

	cores := xslices.Map(dests, func(dest *LogDestination) zapcore.Core {
		core := zapcore.NewCore(
			zapcore.NewConsoleEncoder(dest.config),
			zapcore.AddSync(dest.writer),
			dest.level,
		)

		if dest.suppressThreshold > 0 {
			core = NewControllerErrorSuppressor(core, dest.suppressThreshold)
		}

		return core
	})

	return zap.New(zapcore.NewTee(cores...))
}

// Component helper for creating zap.Field.
func Component(name string) zapcore.Field {
	return zap.String("component", name)
}
