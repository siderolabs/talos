// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runner_test

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
)

// TestLastLogErrorText covers the rendered error text.
func TestLastLogErrorText(t *testing.T) {
	t.Parallel()

	err := &runner.LastLogError{
		Err:     errors.New(`task "web" failed: exit code 1`),
		LastLog: "starting\nopen /x: permission denied \x1b[0m",
	}

	assert.Equal(t, `task "web" failed: exit code 1 (last log "starting\nopen /x: permission denied \x1b[0m")`, err.Error())
}

func TestLastLogErrorUnwrap(t *testing.T) {
	t.Parallel()

	exitErr := &exec.ExitError{}

	assert.ErrorIs(t, &runner.LastLogError{Err: exitErr, LastLog: "boom"}, exitErr)
}
