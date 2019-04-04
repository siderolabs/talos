/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"log"

	"github.com/talos-systems/talos/internal/app/proxyd/internal/frontend"
)

func main() {
	r, err := frontend.NewReverseProxy()
	if err != nil {
		log.Fatalf("failed to initialize the reverse proxy: %v", err)
	}

	// nolint: errcheck
	go r.Watch()

	// nolint: errcheck
	r.Listen(":443")
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}
