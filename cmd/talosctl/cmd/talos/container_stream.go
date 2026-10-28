// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package talos

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"

	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/client"
)

// containerStreamTerminal connects the local terminal to a command streamed from the node: stdout
// and stderr, the forwarded signals and, with stdin, stdin and, with tty, raw mode and terminal size
// changes.
//
// The returned function restores the terminal and must be called once the command has ended.
func containerStreamTerminal(ctx context.Context, stdin, tty bool) (client.ContainerStreamOptions, func(), error) {
	var restoreTerminal func()

	// Raw mode only when the keystrokes are passed on: otherwise the local terminal has to keep
	// turning Ctrl-C into a signal, which is then forwarded.
	if stdin && tty {
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return client.ContainerStreamOptions{}, nil, fmt.Errorf("failed to set terminal to raw mode: %w", err)
		}

		restoreTerminal = func() {
			term.Restore(int(os.Stdin.Fd()), oldState) //nolint:errcheck
		}
	}

	var stdinR io.Reader

	if stdin {
		// os.Stdin itself is never closed, so that the terminal can still be restored.
		pipeR, pipeW := io.Pipe()

		go func() {
			io.Copy(pipeW, os.Stdin) //nolint:errcheck
			pipeW.Close()            //nolint:errcheck
		}()

		stdinR = pipeR
	}

	stop := make(chan struct{})

	var forwarders sync.WaitGroup

	signalC, releaseSignals := forwardSignals(ctx, stop, &forwarders)

	opts := client.ContainerStreamOptions{
		Stdin: stdinR,
		// The command's output is passed through as it is: a terminal session with a pty on the
		// node, or the data the command writes.
		Stdout: os.Stdout, //nolint:forbidigo
		Stderr: os.Stderr, //nolint:forbidigo
		Signal: signalC,
	}

	stopResize := func() {}

	if tty {
		opts.Resize, stopResize = forwardResizes(stop, &forwarders)
	}

	return opts, func() {
		close(stop)
		forwarders.Wait()

		releaseSignals()
		stopResize()

		if restoreTerminal != nil {
			restoreTerminal()
		}
	}, nil
}

// forwardSignals takes over the forwarded signals until stop is closed.
func forwardSignals(ctx context.Context, stop <-chan struct{}, forwarders *sync.WaitGroup) (<-chan syscall.Signal, func()) {
	sigC := make(chan os.Signal, 4)
	release := cli.ClaimSignals(ctx, sigC, syscall.SIGQUIT, syscall.SIGHUP)

	signalC := make(chan syscall.Signal)

	forwarders.Go(func() {
		for {
			select {
			case sig := <-sigC:
				select {
				case signalC <- sig.(syscall.Signal):
				case <-stop:
					return
				}
			case <-stop:
				return
			}
		}
	})

	return signalC, release
}

// forwardResizes reports the terminal size, and every change to it, until stop is closed.
func forwardResizes(stop <-chan struct{}, forwarders *sync.WaitGroup) (<-chan client.TerminalSize, func()) {
	winchC := make(chan os.Signal, 1)
	signal.Notify(winchC, syscall.SIGWINCH)

	winchC <- syscall.SIGWINCH // the initial size

	resizeC := make(chan client.TerminalSize)

	forwarders.Go(func() {
		for {
			select {
			case <-winchC:
				width, height, err := term.GetSize(int(os.Stdin.Fd()))
				if err != nil {
					continue
				}

				select {
				case resizeC <- client.TerminalSize{Width: uint32(width), Height: uint32(height)}: //nolint:gosec
				case <-stop:
					return
				}
			case <-stop:
				return
			}
		}
	})

	return resizeC, func() { signal.Stop(winchC) }
}
