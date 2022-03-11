// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package logind

import "github.com/godbus/dbus/v5"

const (
	dbusPath      = dbus.ObjectPath("/org/freedesktop/DBus")
	dbusInterface = "org.freedesktop.DBus"
)

type dbusMock struct{}

func (dbusMock) Hello() (string, *dbus.Error) {
	return "id", nil
}

func (dbusMock) AddMatch(_ string) *dbus.Error {
	return nil
}
