/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package proc

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
	keyPath := path.Join("/proc/sys", strings.Replace(prop.Key, ".", "/", -1))
	return ioutil.WriteFile(keyPath, []byte(prop.Value), 0644)
}
