// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install_test

import (
	"errors"
	"log"
	"os"

	"github.com/siderolabs/talos/internal/pkg/install"
)

func ExampleWrapOnErr() {
	log.SetFlags(log.Lshortfile)
	log.SetOutput(os.Stdout)

	fn := install.WrapOnErr(alwaysErr, "context for the error")

	defer install.LogError(fn)

	install.LogError(fn)

	// Output:
	// export_test.go:12: context for the error: always an error
}

func alwaysErr() error {
	return errors.New("always an error")
}
