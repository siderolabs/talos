// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package stallguard provides a net.Conn wrapper which aborts reads that make
// no progress for a configurable idle timeout.
package stallguard

import (
	"net"
	"sync"
	"time"
)

// Conn wraps a net.Conn and aborts reads which make no progress for idleTimeout.
//
// The read deadline is armed before every Read and reset on each successful
// read, so a connection trips the deadline only when no bytes flow at all for
// the full idleTimeout window. Slow-but-progressing reads are not penalized.
//
// This guards against connections which are silently black-holed (e.g. after a
// network reconfiguration) and would otherwise hang indefinitely.
//
// Deadlines set by the caller via SetReadDeadline/SetDeadline are honored: the
// effective read deadline is the earlier of the caller-set deadline and the
// stall guard's idle deadline, so the guard never relaxes a shorter timeout the
// caller asked for.
type Conn struct {
	net.Conn

	idleTimeout time.Duration

	mu               sync.Mutex
	externalDeadline time.Time // caller-set read deadline; zero means none
}

// NewConn wraps conn so that reads which make no progress for idleTimeout fail
// with an i/o timeout error.
func NewConn(conn net.Conn, idleTimeout time.Duration) *Conn {
	return &Conn{
		Conn:        conn,
		idleTimeout: idleTimeout,
	}
}

// Read implements io.Reader, arming a read deadline before delegating to the
// underlying connection.
func (c *Conn) Read(b []byte) (int, error) {
	idleDeadline := time.Now().Add(c.idleTimeout)

	c.mu.Lock()
	external := c.externalDeadline
	c.mu.Unlock()

	deadline := idleDeadline
	if !external.IsZero() && external.Before(idleDeadline) {
		deadline = external
	}

	if err := c.Conn.SetReadDeadline(deadline); err != nil {
		return 0, err
	}

	return c.Conn.Read(b)
}

// SetReadDeadline records the caller-set read deadline and applies it to the
// underlying connection, so an in-flight Read is unblocked immediately.
func (c *Conn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.externalDeadline = t
	c.mu.Unlock()

	return c.Conn.SetReadDeadline(t)
}

// SetDeadline records the caller-set read deadline (SetDeadline sets both read
// and write deadlines) and applies it to the underlying connection.
func (c *Conn) SetDeadline(t time.Time) error {
	c.mu.Lock()
	c.externalDeadline = t
	c.mu.Unlock()

	return c.Conn.SetDeadline(t)
}
