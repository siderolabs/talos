// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package logind provides D-Bus logind mock to facilitate graceful kubelet shutdown.
package logind

import (
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/prop"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

const (
	logindService   = "org.freedesktop.login1"
	logindObject    = dbus.ObjectPath("/org/freedesktop/login1")
	logindInterface = "org.freedesktop.login1.Manager"
)

// InhibitMaxDelay is the maximum delay for graceful shutdown.
const InhibitMaxDelay = 40 * constants.KubeletShutdownGracePeriod

type logindMock struct {
	mu          sync.Mutex
	inhibitPipe []int
}

var logindProps = map[string]map[string]*prop.Prop{
	logindInterface: {
		"InhibitDelayMaxUSec": {
			Value:    uint64(InhibitMaxDelay / time.Microsecond),
			Writable: false,
		},
	},
}

func (mock *logindMock) Inhibit(what, who, why, mode string) (dbus.UnixFD, *dbus.Error) {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	for _, fd := range mock.inhibitPipe {
		syscall.Close(fd) //nolint:errcheck
	}

	mock.inhibitPipe = make([]int, 2)
	if err := syscall.Pipe2(mock.inhibitPipe, syscall.O_CLOEXEC); err != nil {
		return dbus.UnixFD(0), dbus.MakeFailedError(err)
	}

	return dbus.UnixFD(mock.inhibitPipe[1]), nil
}

func (mock *logindMock) getPipe() []int {
	mock.mu.Lock()
	defer mock.mu.Unlock()

	return slices.Clone(mock.inhibitPipe)
}
