// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lastlog_test

import (
	"bytes"
	_ "embed"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner/internal/lastlog"
)

//go:embed testdata/kubelet.log
var lastLogData []byte

type readerWrap struct {
	io.Reader
}

func TestLastLog(t *testing.T) {
	t.Parallel()

	for _, step := range []int{1, 7, 13, 29, 37, 64, 128, 256, 512, 1024} {
		t.Run(fmt.Sprintf("step=%d", step), func(t *testing.T) {
			t.Parallel()

			w := &lastlog.Writer{}

			for i := 0; i < len(lastLogData); i += step {
				n := min(step, len(lastLogData)-i)

				out, err := w.Write(lastLogData[i : i+n])
				require.NoError(t, err)
				require.Equal(t, n, out)
			}

			assert.Equal(t, "172.20.0.2: {\"ts\":1758106728112.3425,\"caller\":\"topologymanager/scope.go:117\",\"msg\":\"RemoveContainer\",\"v\":0,\"containerID\":\"d89f847ed0a3500dd712577fcb52dbbc5169bac1b7017d2c6a3f5f809693806c\"}\n172.20.0.2: {\"ts\":1758106728119.877,\"caller\":\"topologymanager/scope.go:117\",\"msg\":\"RemoveContainer\",\"v\":0,\"containerID\":\"56aeb1c5d4785d1d5cc51984dd244fdd4fb672f91e82cfa21e02c92d0f7c3be3\"}", w.GetLastLog()) //nolint:lll
		})
	}
}

func BenchmarkWrites(b *testing.B) {
	buf := make([]byte, 128)

	var r bytes.Reader

	b.ReportAllocs()
	b.ResetTimer()

	w := &lastlog.Writer{}

	for b.Loop() {
		r.Reset(lastLogData)
		src := readerWrap{&r}

		n, err := io.CopyBuffer(w, src, buf)
		require.NoError(b, err)
		require.Equal(b, int64(len(lastLogData)), n)
	}
}
