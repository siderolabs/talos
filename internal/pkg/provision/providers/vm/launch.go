// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
)

// ReadConfig loads configuration from stdin.
func ReadConfig(config interface{}) error {
	d := json.NewDecoder(os.Stdin)
	if err := d.Decode(config); err != nil {
		return fmt.Errorf("error decoding config from stdin: %w", err)
	}

	if d.More() {
		return fmt.Errorf("extra unexpected input on stdin")
	}

	if err := os.Stdin.Close(); err != nil {
		return err
	}

	return nil
}

// ConfigureSignals configures signal handling for the process.
func ConfigureSignals() chan os.Signal {
	signal.Ignore(syscall.SIGHUP)

	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGINT)

	return c
}
