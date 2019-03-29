/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package helpers

import (
	"fmt"
	"os"
	"strings"
)

// Fatalf prints formatted message to stderr and aborts execution
func Fatalf(message string, args ...interface{}) {
	if !strings.HasSuffix(message, "\n") {
		message += "\n"
	}

	fmt.Fprintf(os.Stderr, message, args...)
	os.Exit(1)
}
