/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"os/exec"

	"github.com/armon/circbuf"
	"github.com/pkg/errors"

	"github.com/talos-systems/talos/pkg/proc/reaper"
)

// MaxStderrLen is maximum length of stderr output captured for error message
const MaxStderrLen = 4096

// Run executes a command.
func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)

	stderr, err := circbuf.NewBuffer(MaxStderrLen)
	if err != nil {
		return err
	}
	cmd.Stderr = stderr

	notifyCh := make(chan reaper.ProcessInfo, 8)
	usingReaper := reaper.Notify(notifyCh)
	if usingReaper {
		defer reaper.Stop(notifyCh)
	}

	if err = cmd.Start(); err != nil {
		return errors.Errorf("%s: %s", err, stderr.String())
	}

	if err = reaper.WaitWrapper(usingReaper, notifyCh, cmd); err != nil {
		return errors.Errorf("%s: %s", err, stderr.String())
	}

	return nil
}
