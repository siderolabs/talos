// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logging

import (
	"context"
	"encoding/json"
	"fmt"
	"net"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

type jsonSender struct {
	net  string
	addr string

	sema chan struct{}
	conn net.Conn
}

// NewJSON returns log sender that sends logs in JSON over UDP, one message per packet.
func NewJSON(addr string) runtime.LogSender {
	sema := make(chan struct{}, 1)
	sema <- struct{}{}

	// It should be easy to add TCP support of that's requested.
	net := "udp"

	return &jsonSender{
		net:  net,
		addr: addr,
		sema: sema,
	}
}

func (j jsonSender) tryLock(ctx context.Context) (unlock func()) {
	select {
	case <-j.sema:
		unlock = func() { j.sema <- struct{}{} }
	case <-ctx.Done():
		unlock = nil
	}

	return
}

// Send implements LogSender interface.
func (j *jsonSender) Send(ctx context.Context, e *runtime.LogEvent) error {
	m := make(map[string]interface{}, len(e.Fields)+3)

	// TODO(aleksi): extract fields from msg there or in circularHandler

	m["msg"] = e.Msg
	m["time"] = e.Time.Unix()
	m["level"] = e.Level.String()

	for k, v := range e.Fields {
		m[k] = v
	}

	b, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("%w: %s", runtime.ErrDontRetry, err)
	}

	unlock := j.tryLock(ctx)
	if unlock != nil {
		return ctx.Err()
	}

	defer unlock()

	// Connect (or "connect" for UDP) if no connection is established already.
	if j.conn == nil {
		conn, err := new(net.Dialer).DialContext(ctx, j.net, j.addr)
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

		if n > 0 {
			err = fmt.Errorf("%w: %s", runtime.ErrDontRetry, err)
		}

		return err
	}

	return nil
}

// Close implements LogSender interface.
func (j *jsonSender) Close(ctx context.Context) error {
	unlock := j.tryLock(ctx)
	if unlock != nil {
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
