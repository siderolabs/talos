// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net"
	"net/url"
	"syscall"
	"time"

	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

// tcpUserTimeout bounds how long data sent to the receiver may stay unacknowledged: without it, a receiver
// which goes away without closing the connection is noticed only when TCP gives up retransmitting (~15 minutes).
const tcpUserTimeout = 60 * time.Second

type jsonLinesSender struct {
	endpoint  *url.URL
	extraTags map[string]string

	sema chan struct{}
	conn net.Conn
}

// NewJSONLines returns log sender that sends logs in JSON over TCP (newline-delimited)
// or UDP (one message per packet).
func NewJSONLines(cfg config.LoggingDestination) runtime.LogSender {
	sema := make(chan struct{}, 1)
	sema <- struct{}{}

	return &jsonLinesSender{
		endpoint:  cfg.Endpoint(),
		extraTags: cfg.ExtraTags(),

		sema: sema,
	}
}

func (j *jsonLinesSender) tryLock(ctx context.Context) (unlock func()) {
	select {
	case <-j.sema:
		unlock = func() { j.sema <- struct{}{} }
	case <-ctx.Done():
		unlock = nil
	}

	return unlock
}

func (j *jsonLinesSender) marshalJSON(e *runtime.LogEvent) ([]byte, error) {
	m := make(map[string]any, len(e.Fields)+3)

	maps.Copy(m, e.Fields)
	m["msg"] = e.Msg
	m["talos-time"] = e.Time.Format(time.RFC3339Nano)
	m["talos-level"] = e.Level.String()

	for k, v := range j.extraTags {
		m[k] = v
	}

	return json.Marshal(m)
}

// Send implements LogSender interface.
func (j *jsonLinesSender) Send(ctx context.Context, e *runtime.LogEvent) error {
	b, err := j.marshalJSON(e)
	if err != nil {
		return fmt.Errorf("%w: %s", runtime.ErrDontRetry, err)
	}

	if j.endpoint.Scheme == "tcp" {
		b = append(b, '\n')
	}

	unlock := j.tryLock(ctx)
	if unlock == nil {
		return ctx.Err()
	}

	defer unlock()

	if err = j.connect(ctx); err != nil {
		return err
	}

	d, _ := ctx.Deadline()
	j.conn.SetWriteDeadline(d) //nolint:errcheck

	// Close connection on send error.
	if n, err := j.conn.Write(b); err != nil {
		j.conn.Close() //nolint:errcheck
		j.conn = nil

		// skip partially sent events to avoid partial duplicates in the receiver
		if n > 0 {
			err = fmt.Errorf("%w: %s", runtime.ErrDontRetry, err)
		}

		return err
	}

	return nil
}

// connect establishes the connection (or "connects" for UDP) unless it is already established.
func (j *jsonLinesSender) connect(ctx context.Context) error {
	// Reconnect if the receiver has closed the connection.
	if j.conn != nil && receiverClosed(j.conn) {
		j.conn.Close() //nolint:errcheck
		j.conn = nil
	}

	if j.conn != nil {
		return nil
	}

	var dialer net.Dialer

	if j.endpoint.Scheme == "tcp" {
		dialer.Control = setTCPUserTimeout
	}

	conn, err := dialer.DialContext(ctx, j.endpoint.Scheme, j.endpoint.Host)
	if err != nil {
		return err
	}

	j.conn = conn

	return nil
}

func setTCPUserTimeout(_, _ string, c syscall.RawConn) error {
	var sockErr error

	if err := c.Control(func(fd uintptr) {
		sockErr = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_USER_TIMEOUT, int(tcpUserTimeout.Milliseconds()))
	}); err != nil {
		return err
	}

	if sockErr != nil {
		return fmt.Errorf("failed to set TCP_USER_TIMEOUT: %w", sockErr)
	}

	return nil
}

// receiverClosed reports whether the receiver has closed the TCP connection (or it has failed).
//
// The sender never reads from the connection, so it doesn't see the FIN from the receiver, and writes
// into the closed connection keep succeeding until the receiver responds with a RST, or, if the receiver
// is gone, until TCP gives up retransmitting; anything written until then is lost.
//
// The check is a non-blocking read: EOF means the receiver has closed the connection.
func receiverClosed(conn net.Conn) bool {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return false
	}

	rawConn, err := tcpConn.SyscallConn()
	if err != nil {
		return true
	}

	var closed bool

	if err = rawConn.Read(func(fd uintptr) bool {
		var buf [64]byte

		for {
			n, err := unix.Read(int(fd), buf[:])
			if n > 0 {
				// the receiver is not expected to send anything, discard
				continue
			}

			// n == 0 is EOF, any other error except for EAGAIN means the connection has failed
			closed = !errors.Is(err, unix.EAGAIN)

			return true
		}
	}); err != nil {
		return true
	}

	return closed
}

// Close implements LogSender interface.
func (j *jsonLinesSender) Close(ctx context.Context) error {
	unlock := j.tryLock(ctx)
	if unlock == nil {
		return ctx.Err()
	}

	defer unlock()

	if j.conn == nil {
		return nil
	}

	conn := j.conn
	j.conn = nil

	closed := make(chan error, 1)

	go func() {
		closed <- conn.Close()
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-closed:
		return err
	}
}
