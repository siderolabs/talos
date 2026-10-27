// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bgp_test

import (
	"net"
	"os"
	"testing"

	"github.com/jsimonetti/rtnetlink/v2"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/pkg/testnetns"
)

func TestMain(m *testing.M) {
	os.Exit(testnetns.Run(func() int {
		// GoBGP opens sockets in background goroutines, so isolate the whole
		// process, not just an OS-thread-locked test goroutine.
		conn, err := rtnetlink.Dial(nil)
		if err != nil {
			panic(err)
		}

		lo, err := net.InterfaceByName("lo")
		if err != nil {
			panic(err)
		}

		if err = conn.Link.Set(&rtnetlink.LinkMessage{
			Index: uint32(lo.Index), Flags: unix.IFF_UP, Change: unix.IFF_UP,
		}); err != nil {
			panic(err)
		}

		if err = conn.Close(); err != nil {
			panic(err)
		}

		return m.Run()
	}))
}
