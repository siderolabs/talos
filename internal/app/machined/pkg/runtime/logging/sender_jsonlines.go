// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
)

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
	for k, v := range e.Fields {
		m[k] = v
	}

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

	// Connect (or "connect" for UDP) if no connection is established already.
	if j.conn == nil {
		conn, err := new(net.Dialer).DialContext(ctx, j.endpoint.Scheme, j.endpoint.Host)
		if err != nil {
			return err
		}

		j.conn = conn
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
