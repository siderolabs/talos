// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"bytes"
	"encoding/json"
	"math"
	"strings"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

var maxEpochTS = float64(time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC).Unix())

//nolint:gocyclo
func parseLogLine(l []byte, now time.Time) *runtime.LogEvent {
	msg, m := parseJSONLogLine(l)
	e := &runtime.LogEvent{
		Msg:   msg,
		Time:  now,
		Level: zapcore.InfoLevel,
	}

	if m == nil {
		return e
	}

	for _, k := range []string{"time", "ts"} {
		var t time.Time
		switch ts := m[k].(type) {
		case string:
			t, _ = time.Parse(time.RFC3339Nano, ts) //nolint:errcheck
		case float64:
			// seconds or milliseconds since epoch
			sec, fsec := math.Modf(ts)
			if sec > maxEpochTS {
				sec, fsec = math.Modf(ts / 1000)
			}

			t = time.Unix(int64(sec), int64(fsec*float64(time.Second)))
		}

		if !t.IsZero() {
			e.Time = t.UTC()

			delete(m, k)

			break
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

	if msgS, ok := m["msg"].(string); ok {
		// in case we have both message before JSON and "msg" JSON field
		if e.Msg != "" {
			e.Msg += " "
		}

		e.Msg += strings.TrimSpace(msgS)

		delete(m, "msg")
	}

	if errS, ok := m["err"].(string); ok {
		if e.Level < zap.WarnLevel {
			e.Level = zap.WarnLevel
		}

		if e.Msg != "" {
			e.Msg += ": "
		}

		e.Msg += strings.TrimSpace(errS)

		delete(m, "err")
	}

	e.Fields = m

	return e
}

func parseJSONLogLine(l []byte) (msg string, m map[string]any) {
	// the whole line is valid JSON
	if err := json.Unmarshal(l, &m); err == nil {
		return
	}

	// the line is a message followed by JSON
	if i := bytes.Index(l, []byte("{")); i != -1 {
		if err := json.Unmarshal(l[i:], &m); err == nil {
			msg = string(bytes.TrimSpace(l[:i]))

			return
		}
	}

	// no JSON found
	msg = string(l)

	return
}
