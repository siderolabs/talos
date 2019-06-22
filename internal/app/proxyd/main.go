/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package main

import (
	"flag"
	"log"

	"github.com/talos-systems/talos/internal/app/proxyd/internal/frontend"
	"github.com/talos-systems/talos/pkg/userdata"
)

var (
	dataPath *string
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
	dataPath = flag.String("userdata", "", "the path to the user data")
	flag.Parse()
}

func main() {
	data, err := userdata.Open(*dataPath)
	if err != nil {
		log.Fatalf("open user data: %v", err)
	}

	r, err := frontend.NewReverseProxy(data.Services.Trustd.Endpoints)
	if err != nil {
		log.Fatalf("failed to initialize the reverse proxy: %v", err)
	}

	// nolint: errcheck
	go r.Listen(":443")

	// nolint: errcheck
	r.Watch()
}

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}
