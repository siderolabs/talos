// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package nethelpers

import (
	"fmt"
	"net"
)

type ints interface {
	~int16 | ~int32 | ~int64 | ~uint16 | ~uint32 | ~uint64 | int | uint
}

// JoinHostPort is a wrapper around net.JoinHostPort which accepts port any integer type.
func JoinHostPort[T ints](host string, port T) string {
	return net.JoinHostPort(host, fmt.Sprintf("%d", port))
}
