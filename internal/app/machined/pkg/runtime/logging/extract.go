// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"encoding/json"
	"strings"
	"time"

	"go.uber.org/zap/zapcore"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

func parseLogLine(l []byte, now time.Time) *runtime.LogEvent {
	e := &runtime.LogEvent{
		Msg:   string(l),
		Time:  now,
		Level: zapcore.InfoLevel,
	}

	var m map[string]interface{}
	if err := json.Unmarshal(l, &m); err != nil {
		return e
	}

	if msgS, ok := m["msg"].(string); ok {
		e.Msg = strings.TrimSpace(msgS)

		delete(m, "msg")
	}

	for _, k := range []string{"time", "ts"} {
		if timeS, ok := m[k].(string); ok {
			t, err := time.Parse(time.RFC3339Nano, timeS)
			if err == nil {
				e.Time = t

				delete(m, k)

				break
			}
		}
	}

	if levelS, ok := m["level"].(string); ok {
		levelS = strings.ToLower(levelS)

		// convert containerd's logrus' level to zap's level
		if levelS == "warning" {
			levelS = "warn"
		}

		var level zapcore.Level
		if err := level.UnmarshalText([]byte(levelS)); err == nil {
			e.Level = level

			delete(m, "level")
		}
	}

	e.Fields = m

	return e
}
