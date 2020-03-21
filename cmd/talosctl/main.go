// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"os"

	"github.com/talos-systems/talos/cmd/talosctl/cmd"
	"github.com/talos-systems/talos/pkg/cli"
	"github.com/talos-systems/talos/pkg/startup"
)

func main() {
	cli.Should(startup.RandSeed())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
