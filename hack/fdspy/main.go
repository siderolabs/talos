// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"os"
)

func main() {
	fds, err := os.ReadDir("/proc/self/fd")
	if err != nil {
		panic(err)
	}

	for _, fd := range fds {
		fname, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%s", fd.Name()))
		if err != nil {
			fmt.Fprintln(os.Stderr, fd.Name(), " --> ", err)
		} else {
			fmt.Fprintln(os.Stderr, fd.Name(), " --> ", fname)
		}
	}

	os.Exit(1)
}
