// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package acompat provides compatibility with gRPC 1.67.0 and later.
package acompat

import "os"

func init() {
	if err := os.Setenv("GRPC_ENFORCE_ALPN_ENABLED", "false"); err != nil {
		panic(err)
	}
}
