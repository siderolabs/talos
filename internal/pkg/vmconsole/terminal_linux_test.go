// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vmconsole_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
	"golang.org/x/term"

	"github.com/siderolabs/talos/internal/pkg/vmconsole"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
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

func TestTerminalRestored(t *testing.T) {
	for _, mode := range []string{"SIGTERM", "SIGHUP", "cancel", "detach", "output-error"} {
		t.Run(mode, func(t *testing.T) {
			master, slave := openTerminal(t)
			before, err := term.GetState(int(slave.Fd()))
			require.NoError(t, err)

			ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestTerminalHelper$")

			cmd.Env = append(os.Environ(), "TALOS_CONSOLE_TERMINAL_TEST="+mode)
			cmd.ExtraFiles = []*os.File{slave, master}
			output, err := cmd.CombinedOutput()
			require.NoError(t, err, "%s", output)

			after, err := term.GetState(int(slave.Fd()))
			require.NoError(t, err)
			assert.Equal(t, before, after, "terminal settings were not restored")
		})
	}
}

type terminalWriter struct {
	input  *os.File
	master *os.File
	mode   string
	cancel context.CancelFunc
}

func (w terminalWriter) Write(data []byte) (int, error) {
	state, err := unix.IoctlGetTermios(int(w.input.Fd()), unix.TCGETS)
	if err != nil {
		return 0, err
	}

	if state.Lflag&(unix.ICANON|unix.ECHO|unix.ISIG) != 0 {
		return 0, fmt.Errorf("terminal is not raw: flags %x", state.Lflag)
	}

	switch w.mode {
	case "SIGTERM":
		err = unix.Kill(os.Getpid(), unix.SIGTERM)
	case "SIGHUP":
		err = unix.Kill(os.Getpid(), unix.SIGHUP)
	case "cancel":
		w.cancel()
	case "detach":
		_, err = w.master.Write([]byte{0x1d})
	case "output-error":
		return 0, io.ErrClosedPipe
	}

	return len(data), err
}

func TestTerminalHelper(t *testing.T) {
	mode := os.Getenv("TALOS_CONSOLE_TERMINAL_TEST")
	if mode == "" {
		t.Skip("subprocess helper")
	}

	input := os.NewFile(3, "terminal")
	master := os.NewFile(4, "terminal-master")

	t.Cleanup(func() {
		assert.NoError(t, input.Close())
		assert.NoError(t, master.Close())
	})

	server := &consoleServer{requests: make(chan *machine.ConsoleRequest, 2)}
	client := newConsoleClient(t, server)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	err := vmconsole.Run(ctx, client, "guest", input, terminalWriter{
		input:  input,
		master: master,
		mode:   mode,
		cancel: cancel,
	})

	switch mode {
	case "detach":
		require.NoError(t, err)
	case "output-error":
		require.ErrorIs(t, err, io.ErrClosedPipe)
	default:
		require.ErrorIs(t, err, context.Canceled)
	}
}
