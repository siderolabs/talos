// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"maps"
	"net"
	"net/url"
	"sync/atomic"
	"time"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

type jsonLinesSender struct {
	endpoint  *url.URL
	extraTags map[string]string

	sema chan struct{}
	conn net.Conn

	// connClosed is closed once conn is closed (TCP only).
	connClosed <-chan struct{}

	// closed is set by Close; a closed sender doesn't connect again.
	closed atomic.Bool
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

	// Close might have given up waiting for this Send, so close the connection on its behalf.
	defer func() {
		if j.closed.Load() && j.conn != nil {
			j.conn.Close() //nolint:errcheck
			j.conn = nil
		}
	}()

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
	if j.closed.Load() {
		return errors.New("log sender is closed")
	}

	// Reconnect if the receiver has closed the connection.
	select {
	case <-j.connClosed:
		j.conn = nil
	default:
	}

	if j.conn != nil {
		return nil
	}

	conn, err := new(net.Dialer).DialContext(ctx, j.endpoint.Scheme, j.endpoint.Host)
	if err != nil {
		return err
	}

	j.conn = conn

	if j.endpoint.Scheme == "tcp" {
		j.connClosed = closeOnEOF(conn)
	}

	return nil
}

// closeOnEOF closes the connection once the receiver closes it (or it fails), so that the sender
// doesn't keep writing into it, and returns a channel which is closed at that point.
func closeOnEOF(conn net.Conn) <-chan struct{} {
	closed := make(chan struct{})

	go func() {
		io.Copy(io.Discard, conn) //nolint:errcheck

		// signal first, so that once the connection is seen closed, the next Send reconnects
		close(closed)
		conn.Close() //nolint:errcheck
	}()

	return closed
}

// Close implements LogSender interface.
func (j *jsonLinesSender) Close(ctx context.Context) error {
	j.closed.Store(true)

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
		err := conn.Close()

		// the connection is already closed if the receiver has closed it
		if errors.Is(err, net.ErrClosed) {
			err = nil
		}

		closed <- err
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-closed:
		return err
	}
}
