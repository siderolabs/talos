// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package sysctl

import (
	"io/ioutil"
	"path"
	"strings"
)

// SystemProperty represents a kernel system property.
type SystemProperty struct {
	Key   string
	Value string
}

// WriteSystemProperty writes a value to a key under /proc/sys.
func WriteSystemProperty(prop *SystemProperty) error {
	return ioutil.WriteFile(prop.Path(), []byte(prop.Value), 0o644)
}

// ReadSystemProperty reads a value from a key under /proc/sys.
func ReadSystemProperty(prop *SystemProperty) ([]byte, error) {
	return ioutil.ReadFile(prop.Path())
}

// Path returns the path to the systctl file under /proc/sys.
func (prop *SystemProperty) Path() string {
	return path.Join("/proc/sys", strings.ReplaceAll(prop.Key, ".", "/"))
}
