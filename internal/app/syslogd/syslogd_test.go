// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package syslogd_test

import (
	"context"
	"encoding/json"
	"log/syslog"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/syslogd"
)

type chanWriter struct {
	ch chan []byte
}

func (w *chanWriter) Write(p []byte) (n int, err error) {
	w.ch <- p

	return len(p), nil
}

func TestParsing(t *testing.T) {
	ch := chanWriter{
		ch: make(chan []byte),
	}

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)

	defer cancel()

	socketPath := t.TempDir() + "/syslogd.sock"

	errChan := make(chan error)

	go func() {
		errChan <- syslogd.Run(ctx, &ch, socketPath)
	}()

	assert.Eventually(t, func() bool {
		if _, err := os.Stat(socketPath); err == nil {
			return true
		}

		return false
	}, 1*time.Second, 100*time.Millisecond)

	// Send a message to the syslogd service
	syslog, err := syslog.Dial("unixgram", socketPath, syslog.LOG_INFO, "syslogd_test")
	require.NoError(t, err)

	_, err = syslog.Write([]byte("Hello, syslogd!"))
	require.NoError(t, err)

	defer syslog.Close() //nolint:errcheck

	select {
	case msg := <-ch.ch:
		var parsed map[string]any

		require.NoError(t, json.Unmarshal(msg, &parsed))

		// {"content":"Hello, syslogd!\n","facility":0,"hostname":"localhost","priority":6,"severity":6,"tag":"syslogd_test","timestamp":"2024-02-20T00:20:55Z"
		assert.Equal(t, "syslogd_test", parsed["tag"])
		assert.Equal(t, "localhost", parsed["hostname"])
		assert.Equal(t, float64(6), parsed["priority"])
		assert.Equal(t, float64(6), parsed["severity"])
		assert.Equal(t, float64(0), parsed["facility"])
		assert.Equal(t, "Hello, syslogd!\n", parsed["content"])
		assert.NotEmpty(t, parsed["timestamp"])
	case <-ctx.Done():
		require.Fail(t, "timed out waiting for message")
	}

	cancel()

	require.NoError(t, <-errChan)
}
