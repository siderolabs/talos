/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"bytes"
	"os/exec"

	"github.com/pkg/errors"
)

// Run executes a command.
func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Start(); err != nil {
		return errors.Errorf("%s: %s", err, stderr.String())
	}

	if err := cmd.Wait(); err != nil {
		return errors.Errorf("%s: %s", err, stderr.String())
	}

	return nil
}
