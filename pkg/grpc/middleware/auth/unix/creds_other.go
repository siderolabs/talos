// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !linux

package unix

import (
	"google.golang.org/grpc/credentials"
)

// NewServerCredentials is not supported on non-Linux platforms.
func NewServerCredentials() credentials.TransportCredentials {
	panic("unix socket peer credentials are only supported on Linux")
}
