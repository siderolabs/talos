// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging //nolint:testpackage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
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
		"controller-runtime": {
			l: `reconfigured wireguard link {"component": "controller-runtime", "controller": "network.LinkSpecController", "link": "kubespan", "peers": 4}`,
			expected: &runtime.LogEvent{
				Msg:   `reconfigured wireguard link`,
				Time:  now,
				Level: zapcore.InfoLevel,
				Fields: map[string]interface{}{
					"component":  "controller-runtime",
					"controller": "network.LinkSpecController",
					"link":       "kubespan",
					"peers":      float64(4),
				},
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
		"kubelet": {
			l: `{"ts":1635266764792.703,"caller":"topologymanager/scope.go:110","msg":"RemoveContainer","v":0,"containerID":"0194fac91ac1d3949497f6912f3c7e73a062c3bf29b6d3da05557d4db2f8482b"}`,
			expected: &runtime.LogEvent{
				Msg:   `RemoveContainer`,
				Time:  time.Date(2021, 10, 26, 16, 46, 4, 792702913, time.UTC),
				Level: zapcore.InfoLevel,
				Fields: map[string]interface{}{
					"caller":      "topologymanager/scope.go:110",
					"containerID": "0194fac91ac1d3949497f6912f3c7e73a062c3bf29b6d3da05557d4db2f8482b",
					"v":           float64(0),
				},
			},
		},
		"kubelet-err": {
			l: `{"ts":1635266751595.943,"caller":"kubelet/kubelet.go:1703","msg":"Failed creating a mirror pod for",` +
				`"pod":"kube-system/kube-controller-manager-talos-dev-qemu-master-1","err":"pods \"kube-controller-manager-talos-dev-qemu-master-1\" already exists"}`,
			expected: &runtime.LogEvent{
				Msg:   `Failed creating a mirror pod for: pods "kube-controller-manager-talos-dev-qemu-master-1" already exists`,
				Time:  time.Date(2021, 10, 26, 16, 45, 51, 595943212, time.UTC),
				Level: zapcore.WarnLevel,
				Fields: map[string]interface{}{
					"caller": "kubelet/kubelet.go:1703",
					"pod":    "kube-system/kube-controller-manager-talos-dev-qemu-master-1",
				},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			actual := parseLogLine([]byte(tc.l), now)
			assert.Equal(t, tc.expected, actual)
		})
	}
}
