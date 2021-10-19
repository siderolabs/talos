// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging //nolint:testpackage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

func TestParseLogLine(t *testing.T) {
	t.Parallel()

	now := time.Date(2021, 10, 19, 12, 42, 37, 123456789, time.UTC)

	for name, tc := range map[string]struct {
		l        string
		expected *runtime.LogEvent
	}{
		"machined": {
			l: `[talos] task updateBootloader (1/1): done, 219.885384ms`,
			expected: &runtime.LogEvent{
				Msg:   `[talos] task updateBootloader (1/1): done, 219.885384ms`,
				Time:  now,
				Level: zapcore.InfoLevel,
			},
		},
		"etcd-zap": {
			l: `{"level":"info","ts":"2021-10-19T14:53:05.815Z","caller":"mvcc/kvstore_compaction.go:57","msg":"finished scheduled compaction","compact-revision":34567,"took":"21.041639ms"}`,
			expected: &runtime.LogEvent{
				Msg:   `finished scheduled compaction`,
				Time:  time.Date(2021, 10, 19, 14, 53, 5, 815000000, time.UTC),
				Level: zapcore.InfoLevel,
				Fields: map[string]interface{}{
					"caller":           "mvcc/kvstore_compaction.go:57",
					"compact-revision": float64(34567),
					"took":             "21.041639ms",
				},
			},
		},
		"cri-logrus": {
			l: `{"level":"warning","msg":"cleanup warnings time=\"2021-10-19T14:52:20Z\" level=info msg=\"starting signal loop\" namespace=k8s.io pid=2629\n","time":"2021-10-19T14:52:20.578858689Z"}`,
			expected: &runtime.LogEvent{
				Msg:    `cleanup warnings time="2021-10-19T14:52:20Z" level=info msg="starting signal loop" namespace=k8s.io pid=2629`,
				Time:   time.Date(2021, 10, 19, 14, 52, 20, 578858689, time.UTC),
				Level:  zapcore.WarnLevel,
				Fields: map[string]interface{}{},
			},
		},
	} {
		name, tc := name, tc
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual := parseLogLine([]byte(tc.l), now)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
