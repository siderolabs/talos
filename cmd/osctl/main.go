/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"github.com/talos-systems/talos/cmd/osctl/cmd"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/internal/pkg/startup"
)

func main() {
	helpers.Should(startup.RandSeed())

	cmd.Execute()
}
