// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging_test

import (
	"bufio"
	"context"
	"encoding/json"
	"net"
	"net/url"
	"sync"
	"testing"
	"time"

	"github.com/siderolabs/gen/channel"
	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/logging"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func udpHandler(ctx context.Context, t *testing.T, conn net.PacketConn, sendCh chan<- []byte) {
	t.Helper()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
			t.Logf("failed to set read deadline: %v", err)

			return
		}

		buf := make([]byte, 1024)

		n, _, err := conn.ReadFrom(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			t.Logf("failed to read from UDP connection: %v", err)

			return
		}

		if !channel.SendWithContext(ctx, sendCh, buf[:n]) {
			return
		}
	}
}

func tcpHandler(ctx context.Context, t *testing.T, conn net.Listener, sendCh chan<- []byte) {
	t.Helper()

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := conn.(*net.TCPListener).SetDeadline(time.Now().Add(10 * time.Millisecond)); err != nil {
			t.Logf("failed to set accept deadline: %v", err)

			return
		}

		c, err := conn.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}

			t.Logf("failed to accept UDP connection: %v", err)

			return
		}

		go func() {
			defer c.Close() //nolint:errcheck

			scanner := bufio.NewScanner(c)

			for scanner.Scan() {
				if !channel.SendWithContext(ctx, sendCh, scanner.Bytes()) {
					return
				}
			}
		}()
	}
}

type loggingDestination struct {
	endpoint  *url.URL
	extraTags map[string]string
}

func (l *loggingDestination) Endpoint() *url.URL {
	return l.endpoint
}

func (l *loggingDestination) ExtraTags() map[string]string {
	return l.extraTags
}

func (l *loggingDestination) Format() string {
	return constants.LoggingFormatJSONLines
}

func TestSenderJSONLines(t *testing.T) { //nolint:tparallel
	t.Parallel()

	lisUDP, err := (&net.ListenConfig{}).ListenPacket(t.Context(), "udp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, lisUDP.Close())
	})

	lisTCP, err := (&net.ListenConfig{}).Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, lisTCP.Close())
	})

	udpEndpoint := lisUDP.LocalAddr().String()
	tcpEndpoint := lisTCP.Addr().String()

	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	t.Cleanup(cancel)

	sendCh := make(chan []byte, 32)

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		udpHandler(ctx, t, lisUDP, sendCh)
	}()

	wg.Add(1)

	go func() {
		defer wg.Done()

		tcpHandler(ctx, t, lisTCP, sendCh)
	}()

	t.Cleanup(wg.Wait)

	for _, test := range []struct {
		name string

		endpoint  *url.URL
		extraTags map[string]string

		messages []*runtime.LogEvent

		expected []map[string]any
	}{
		{
			name: "UDP",

			endpoint: ensure.Value(url.Parse("udp://" + udpEndpoint)),

			messages: []*runtime.LogEvent{
				{
					Msg:   "msg1",
					Time:  ensure.Value(time.Parse(time.RFC3339Nano, "2021-01-01T00:00:00Z")),
					Level: zapcore.InfoLevel,
					Fields: map[string]any{
						"field1": "value1",
					},
				},
				{
					Msg:   "msg2",
					Time:  ensure.Value(time.Parse(time.RFC3339Nano, "2021-01-01T00:00:01Z")),
					Level: zapcore.DebugLevel,
				},
			},

			expected: []map[string]any{
				{
					"field1":      "value1",
					"msg":         "msg1",
					"talos-level": "info",
					"talos-time":  "2021-01-01T00:00:00Z",
				},
				{
					"msg":         "msg2",
					"talos-level": "debug",
					"talos-time":  "2021-01-01T00:00:01Z",
				},
			},
		},
		{
			name: "UDP with extra tags",

			endpoint: ensure.Value(url.Parse("udp://" + udpEndpoint)),
			extraTags: map[string]string{
				"extra1": "value1",
			},

			messages: []*runtime.LogEvent{
				{
					Msg:   "msg1",
					Time:  ensure.Value(time.Parse(time.RFC3339Nano, "2021-01-01T00:00:00Z")),
					Level: zapcore.InfoLevel,
					Fields: map[string]any{
						"field1": "value1",
					},
				},
				{
					Msg:   "msg2",
					Time:  ensure.Value(time.Parse(time.RFC3339Nano, "2021-01-01T00:00:01Z")),
					Level: zapcore.DebugLevel,
				},
			},

			expected: []map[string]any{
				{
					"field1":      "value1",
					"extra1":      "value1",
					"msg":         "msg1",
					"talos-level": "info",
					"talos-time":  "2021-01-01T00:00:00Z",
				},
				{
					"msg":         "msg2",
					"extra1":      "value1",
					"talos-level": "debug",
					"talos-time":  "2021-01-01T00:00:01Z",
				},
			},
		},
		{
			name: "TCP",

			endpoint: ensure.Value(url.Parse("tcp://" + tcpEndpoint)),

			messages: []*runtime.LogEvent{
				{
					Msg:   "hello",
					Time:  ensure.Value(time.Parse(time.RFC3339Nano, "2021-01-01T00:00:00Z")),
					Level: zapcore.InfoLevel,
					Fields: map[string]any{
						"field1": "value1",
					},
				},
			},

			expected: []map[string]any{
				{
					"field1":      "value1",
					"msg":         "hello",
					"talos-level": "info",
					"talos-time":  "2021-01-01T00:00:00Z",
				},
			},
		},
		{
			name: "TCP with extra tags",

			endpoint: ensure.Value(url.Parse("tcp://" + tcpEndpoint)),
			extraTags: map[string]string{
				"extra1": "value1",
			},

			messages: []*runtime.LogEvent{
				{
					Msg:   "hello",
					Time:  ensure.Value(time.Parse(time.RFC3339Nano, "2021-01-01T00:00:00Z")),
					Level: zapcore.InfoLevel,
					Fields: map[string]any{
						"field1": "value1",
					},
				},
			},

			expected: []map[string]any{
				{
					"field1":      "value1",
					"extra1":      "value1",
					"msg":         "hello",
					"talos-level": "info",
					"talos-time":  "2021-01-01T00:00:00Z",
				},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// not parallel - need sequential execution
			loggingCfg := &loggingDestination{
				endpoint:  test.endpoint,
				extraTags: test.extraTags,
			}

			sender := logging.NewJSONLines(loggingCfg)

			for _, msg := range test.messages {
				require.NoError(t, sender.Send(ctx, msg))
			}

			for _, expected := range test.expected {
				select {
				case <-time.After(time.Second):
					t.Fatalf("timed out waiting for message")
				case msg := <-sendCh:
					var m map[string]any

					require.NoError(t, json.Unmarshal(msg, &m))

					require.Equal(t, expected, m)
				}
			}

			require.NoError(t, sender.Close(ctx))
		})
	}

	cancel()
}
