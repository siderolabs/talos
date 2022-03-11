// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logind

import (
	"context"
	"syscall"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"
)

// ServiceMock connects to the broker and mocks the D-Bus and logind.
type ServiceMock struct {
	conn   *dbus.Conn
	logind logindMock
}

// NewServiceMock initializes the D-Bus and logind mock.
func NewServiceMock(socketPath string) (*ServiceMock, error) {
	var mock ServiceMock

	conn, err := dbus.Dial("unix:path=" + socketPath)
	if err != nil {
		return nil, err
	}

	if err = conn.Auth(nil); err != nil {
		return nil, err
	}

	if err = conn.Export(dbusMock{}, dbusPath, dbusInterface); err != nil {
		return nil, err
	}

	if err = conn.Export(&mock.logind, logindObject, logindService); err != nil {
		return nil, err
	}

	if err = conn.Export(&mock.logind, logindObject, logindInterface); err != nil {
		return nil, err
	}

	_, err = prop.Export(conn, logindObject, logindProps)
	if err != nil {
		return nil, err
	}

	mock.conn = conn

	return &mock, nil
}

// Close the connection.
func (mock *ServiceMock) Close() error {
	return mock.conn.Close()
}

// EmitShutdown notifies about the shutdown.
func (mock *ServiceMock) EmitShutdown() error {
	return mock.conn.Emit(logindObject, logindService+".PrepareForShutdown", true)
}

// WaitLockRelease waits for the inhibit lock to be released.
func (mock *ServiceMock) WaitLockRelease(ctx context.Context) error {
	pipe := mock.logind.getPipe()

	// no inhibit lock
	if len(pipe) == 0 {
		return nil
	}

	// close the write side of the pipe, other fd to the write pipe is in the kubelet
	if err := syscall.Close(pipe[1]); err != nil {
		return err
	}

	errCh := make(chan error, 1)

	go func() {
		// attempt to read from the pipe, as soon as kubelet closes its end, read should return
		buf := make([]byte, 1)
		_, err := syscall.Read(pipe[0], buf)

		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
