// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package terminal_test

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"github.com/siderolabs/talos/internal/pkg/terminal"
)

func openTerminal(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, master.Close()) })
	require.NoError(t, unix.IoctlSetPointerInt(int(master.Fd()), unix.TIOCSPTLCK, 0))

	number, err := unix.IoctlGetInt(int(master.Fd()), unix.TIOCGPTN)
	require.NoError(t, err)

	slave, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, slave.Close()) })

	return master, slave
}

func TestRawStateRestoresTerminal(t *testing.T) {
	_, input := openTerminal(t)

	before, err := term.GetState(int(input.Fd()))
	require.NoError(t, err)

	state, err := terminal.MakeRaw(input)
	require.NoError(t, err)

	raw, err := term.GetState(int(input.Fd()))
	require.NoError(t, err)
	assert.NotEqual(t, before, raw)

	require.NoError(t, state.Restore())
	require.NoError(t, state.Restore())

	after, err := term.GetState(int(input.Fd()))
	require.NoError(t, err)
	assert.Equal(t, before, after)
}
