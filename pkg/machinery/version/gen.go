// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build generate

package main

import (
	"log"
	"os"

	"github.com/siderolabs/talos/pkg/machinery/version"
)

func main() {
	data, err := version.OSRelease()
	if err != nil {
		log.Fatal(err)
	}

	if err = os.WriteFile("os-release", data, 0o644); err != nil {
		log.Fatal(err)
	}
}
