// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/armon/circbuf"

	"github.com/talos-systems/talos/pkg/proc/reaper"
)

// MaxStderrLen is maximum length of stderr output captured for error message.
const MaxStderrLen = 4096

// Run executes a command.
func Run(name string, args ...string) (string, error) {
	return RunContext(context.Background(), name, args...)
}

// RunContext executes a command with context.
func RunContext(ctx context.Context, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)

	stdout, err := circbuf.NewBuffer(MaxStderrLen)
	if err != nil {
		return stdout.String(), err
	}

	stderr, err := circbuf.NewBuffer(MaxStderrLen)
	if err != nil {
		return stdout.String(), err
	}

	cmd.Stdout = stdout
	cmd.Stderr = stderr

	notifyCh := make(chan reaper.ProcessInfo, 8)
	usingReaper := reaper.Notify(notifyCh)

	if usingReaper {
		defer reaper.Stop(notifyCh)
	}

	if err = cmd.Start(); err != nil {
		return stdout.String(), fmt.Errorf("%s: %s", err, stderr.String())
	}

	if err = reaper.WaitWrapper(usingReaper, notifyCh, cmd); err != nil {
		return stdout.String(), fmt.Errorf("%s: %s", err, stderr.String())
	}

	return stdout.String(), nil
}
