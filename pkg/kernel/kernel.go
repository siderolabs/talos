// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kernel

import (
	"io/ioutil"
	"os"
	"path"
	"strings"
)

// Param represents a kernel system property.
type Param struct {
	Key   string
	Value string
}

// WriteParam writes a value to a key under /proc/sys.
func WriteParam(prop *Param) error {
	return ioutil.WriteFile(prop.Path(), []byte(prop.Value), 0o644)
}

// ReadParam reads a value from a key under /proc/sys.
func ReadParam(prop *Param) ([]byte, error) {
	return ioutil.ReadFile(prop.Path())
}

// DeleteParam deletes a value from a key under /proc/sys.
func DeleteParam(prop *Param) error {
	return os.Remove(prop.Path())
}

// Path returns the path to the systctl file under /proc/sys.
func (prop *Param) Path() string {
	return path.Join("/proc/sys", strings.ReplaceAll(prop.Key, ".", "/"))
}
