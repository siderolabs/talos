// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"sync/atomic"

	"go.uber.org/zap/zapcore"
)

// NewControllerErrorSuppressor creates a new controller error suppressor.
//
// It suppresses error logs for a given controller unless it logs >= threshold errors.
// The idea is that all controllers reconcile errors, so if the error is not transient,
// it will be reported enough time to hit the threshold, but transient errors will be
// suppressed.
//
// The suppressor records the controller name by inspecting a log field named "controller"
// passed via `logger.With()` call.
func NewControllerErrorSuppressor(core zapcore.Core, threshold int64) zapcore.Core {
	return &consoleSampler{
		Core:      core,
		threshold: threshold,
	}
}

type consoleSampler struct {
	zapcore.Core

	hits       *atomic.Int64
	threshold  int64
	controller string
}

var _ zapcore.Core = (*consoleSampler)(nil)

func (s *consoleSampler) Level() zapcore.Level {
	return zapcore.LevelOf(s.Core)
}

func (s *consoleSampler) With(fields []zapcore.Field) zapcore.Core {
	controller := s.controller
	num := s.hits

	for _, field := range fields {
		if field.Key == "controller" {
			if field.String != controller {
				controller = field.String
				num = new(atomic.Int64)
			}

			break
		}
	}

	return &consoleSampler{
		threshold:  s.threshold,
		controller: controller,
		hits:       num,
		Core:       s.Core.With(fields),
	}
}

func (s *consoleSampler) Check(ent zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if !s.Enabled(ent.Level) {
		return ce
	}

	if ent.Level == zapcore.ErrorLevel && s.controller != "" {
		if s.hits.Add(1) <= s.threshold {
			// suppress the log
			return ce
		}
	}

	return s.Core.Check(ent, ce)
}
