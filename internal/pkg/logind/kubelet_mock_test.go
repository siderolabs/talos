// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logind_test

import (
	"errors"
	"fmt"
	"log"
	"syscall"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	logindService   = "org.freedesktop.login1"
	logindObject    = dbus.ObjectPath("/org/freedesktop/login1")
	logindInterface = "org.freedesktop.login1.Manager"
)

type dBusConnector interface {
	Object(dest string, path dbus.ObjectPath) dbus.BusObject
	AddMatchSignal(options ...dbus.MatchOption) error
	Signal(ch chan<- *dbus.Signal)
	Close() error
}

// DBusCon has functions that can be used to interact with systemd and logind over dbus.
type DBusCon struct {
	SystemBus dBusConnector
}

func NewDBusCon(path string) (*DBusCon, error) {
	conn, err := dbus.Connect(path)
	if err != nil {
		return nil, err
	}

	return &DBusCon{
		SystemBus: conn,
	}, nil
}

func (bus *DBusCon) Close() error {
	return bus.SystemBus.Close()
}

// InhibitLock is a lock obtained after creating an systemd inhibitor by calling InhibitShutdown().
type InhibitLock uint32

// CurrentInhibitDelay returns the current delay inhibitor timeout value as configured in logind.conf(5).
// see https://www.freedesktop.org/software/systemd/man/logind.conf.html for more details.
func (bus *DBusCon) CurrentInhibitDelay() (time.Duration, error) {
	obj := bus.SystemBus.Object(logindService, logindObject)

	res, err := obj.GetProperty(logindInterface + ".InhibitDelayMaxUSec")
	if err != nil {
		return 0, fmt.Errorf("failed reading InhibitDelayMaxUSec property from logind: %w", err)
	}

	delay, ok := res.Value().(uint64)
	if !ok {
		return 0, errors.New("InhibitDelayMaxUSec from logind is not a uint64 as expected")
	}

	// InhibitDelayMaxUSec is in microseconds
	duration := time.Duration(delay) * time.Microsecond

	return duration, nil
}

// InhibitShutdown creates an systemd inhibitor by calling logind's Inhibt() and returns the inhibitor lock
// see https://www.freedesktop.org/wiki/Software/systemd/inhibit/ for more details.
func (bus *DBusCon) InhibitShutdown() (InhibitLock, error) {
	obj := bus.SystemBus.Object(logindService, logindObject)
	what := "shutdown"
	who := "kubelet"
	why := "Kubelet needs time to handle node shutdown"
	mode := "delay"

	call := obj.Call("org.freedesktop.login1.Manager.Inhibit", 0, what, who, why, mode)
	if call.Err != nil {
		return InhibitLock(0), fmt.Errorf("failed creating systemd inhibitor: %w", call.Err)
	}

	var fd uint32

	err := call.Store(&fd)
	if err != nil {
		return InhibitLock(0), fmt.Errorf("failed storing inhibit lock file descriptor: %w", err)
	}

	return InhibitLock(fd), nil
}

// ReleaseInhibitLock will release the underlying inhibit lock which will cause the shutdown to start.
func (bus *DBusCon) ReleaseInhibitLock(lock InhibitLock) error {
	err := syscall.Close(int(lock))
	if err != nil {
		return fmt.Errorf("unable to close systemd inhibitor lock: %w", err)
	}

	return nil
}

// MonitorShutdown detects the a node shutdown by watching for "PrepareForShutdown" logind events.
// see https://www.freedesktop.org/wiki/Software/systemd/inhibit/ for more details.
func (bus *DBusCon) MonitorShutdown() (<-chan bool, error) {
	err := bus.SystemBus.AddMatchSignal(dbus.WithMatchInterface(logindInterface), dbus.WithMatchMember("PrepareForShutdown"), dbus.WithMatchObjectPath("/org/freedesktop/login1"))
	if err != nil {
		return nil, err
	}

	busChan := make(chan *dbus.Signal, 1)
	bus.SystemBus.Signal(busChan)

	shutdownChan := make(chan bool, 1)

	go func() {
		for {
			event, ok := <-busChan
			if !ok {
				close(shutdownChan)

				return
			}

			if event == nil || len(event.Body) == 0 {
				log.Printf("failed obtaining shutdown event, PrepareForShutdown event was empty")

				continue
			}

			shutdownActive, ok := event.Body[0].(bool)
			if !ok {
				log.Printf("Failed obtaining shutdown event, PrepareForShutdown event was not bool type as expected")

				continue
			}

			shutdownChan <- shutdownActive
		}
	}()

	return shutdownChan, nil
}
