// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/talos-systems/talos/internal/pkg/logind"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// DBusState implements the logind mock.
type DBusState struct {
	broker     *logind.DBusBroker
	logindMock *logind.ServiceMock
	errCh      chan error
	cancel     context.CancelFunc
}

// Start the D-Bus broker and logind mock.
func (dbus *DBusState) Start() error {
	for _, path := range []string{constants.DBusServiceSocketPath, constants.DBusClientSocketPath} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return err
		}
	}

	var err error

	dbus.broker, err = logind.NewBroker(constants.DBusServiceSocketPath, constants.DBusClientSocketPath)
	if err != nil {
		return err
	}

	var ctx context.Context

	ctx, dbus.cancel = context.WithCancel(context.Background())

	dbus.errCh = make(chan error)

	go func() {
		dbus.errCh <- dbus.broker.Run(ctx)
	}()

	dbus.logindMock, err = logind.NewServiceMock(constants.DBusServiceSocketPath)

	return err
}

// Stop the D-Bus broker and logind mock.
func (dbus *DBusState) Stop() error {
	dbus.cancel()

	if err := dbus.logindMock.Close(); err != nil {
		return err
	}

	if err := dbus.broker.Close(); err != nil {
		return err
	}

	select {
	case <-time.After(time.Second):
		return fmt.Errorf("timed out stopping D-Bus broker")
	case err := <-dbus.errCh:
		return err
	}
}

// WaitShutdown signals the shutdown over the D-Bus and waits for the inhibit lock to be released.
func (dbus *DBusState) WaitShutdown(ctx context.Context) error {
	if err := dbus.logindMock.EmitShutdown(); err != nil {
		return err
	}

	return dbus.logindMock.WaitLockRelease(ctx)
}
