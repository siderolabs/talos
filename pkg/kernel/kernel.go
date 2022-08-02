// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kernel

import (
	"os"

	"github.com/talos-systems/talos/pkg/machinery/kernel"
)

// WriteParam writes a value to a key under /proc/sys.
func WriteParam(prop *kernel.Param) error {
	return os.WriteFile(prop.Path(), []byte(prop.Value), 0o644)
}

// ReadParam reads a value from a key under /proc/sys.
func ReadParam(prop *kernel.Param) ([]byte, error) {
	return os.ReadFile(prop.Path())
}

// DeleteParam deletes a value from a key under /proc/sys.
func DeleteParam(prop *kernel.Param) error {
	return os.Remove(prop.Path())
}
