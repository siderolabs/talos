// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bootloader

import (
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

// Bootloader describes a bootloader.
type Bootloader interface {
	Labels() (string, string, error)
	Install(string, interface{}, runtime.Sequence) error
	Default(string) error
}
