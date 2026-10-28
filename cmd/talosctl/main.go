// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package talosctl provides the talosctl utility implementation.
package main

import (
	"errors"
	"os"

	_ "github.com/siderolabs/talos/cmd/talosctl/acompat"
	"github.com/siderolabs/talos/cmd/talosctl/cmd"
	"github.com/siderolabs/talos/cmd/talosctl/cmd/common"
)

func main() {
	if err := cmd.Execute(); err != nil {
		if exitErr, ok := errors.AsType[*common.ExitError](err); ok {
			os.Exit(exitErr.Code)
		}

		os.Exit(1)
	}
}
