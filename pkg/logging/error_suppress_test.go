// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/pkg/logging"
)

func assertLogged(t *testing.T, core zapcore.Core, logs *observer.ObservedLogs, entries []zapcore.Entry, expectedCount int) {
	t.Helper()

	for _, entry := range entries {
		if ce := core.Check(entry, nil); ce != nil {
			ce.Write()
		}
	}

	assert.Len(t, logs.TakeAll(), expectedCount)
}

func TestErrorSuppressor(t *testing.T) {
	t.Parallel()

	core, logs := observer.New(zapcore.InfoLevel)

	const threshold = 2

	core = logging.NewControllerErrorSuppressor(core, threshold)

	// warn/info messages are not affected
	assertLogged(t, core, logs, []zapcore.Entry{
		{Level: zapcore.InfoLevel, Message: "abc"},
		{Level: zapcore.WarnLevel, Message: "def"},
		{Level: zapcore.DebugLevel, Message: "message"}, // below level
	}, 2)

	// different controllers, suppress counters are independent
	controllerCore1 := core.With([]zapcore.Field{{Key: "controller", String: "c1"}})
	controllerCore2 := core.With([]zapcore.Field{{Key: "controller", String: "c2"}})

	assertLogged(t, controllerCore1, logs, []zapcore.Entry{
		{Level: zapcore.ErrorLevel, Message: "controller failed"}, // suppressed
		{Level: zapcore.ErrorLevel, Message: "controller failed"}, // suppressed
		{Level: zapcore.ErrorLevel, Message: "controller failed"},
	}, 1)

	assertLogged(t, controllerCore2, logs, []zapcore.Entry{
		{Level: zapcore.ErrorLevel, Message: "controller failed"}, // suppressed
		{Level: zapcore.ErrorLevel, Message: "controller failed"}, // suppressed
	}, 0)

	assertLogged(t, controllerCore1, logs, []zapcore.Entry{
		{Level: zapcore.ErrorLevel, Message: "controller failed"}, // not suppressed, over threshold
	}, 1)

	assertLogged(t, controllerCore1.With([]zapcore.Field{{Key: "foo", String: "bar"}}), logs, []zapcore.Entry{
		{Level: zapcore.ErrorLevel, Message: "controller failed"}, // .With() without 'controller' field keeps the counter value from the parent
	}, 1)
}
