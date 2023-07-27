// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package installer provides the installer implementation.
package main

import (
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/cmd/installer/cmd/imager"
	"github.com/siderolabs/talos/cmd/installer/cmd/installer"
)

func main() {
	switch filepath.Base(os.Args[0]) {
	case "imager":
		imager.Execute()
	default:
		installer.Execute()
	}
}
