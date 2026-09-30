// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !windows

package terminal

import (
	"fmt"
	"os"
	"sync"

	"golang.org/x/term"
)

// State owns restoration of terminal settings changed by MakeRaw.
type State struct {
	fd       int
	original *term.State
	once     sync.Once
	err      error
}

// IsTerminal reports whether input refers to a terminal.
func IsTerminal(input *os.File) bool {
	return input != nil && term.IsTerminal(int(input.Fd()))
}

// MakeRaw switches a terminal input to raw mode. Non-terminal inputs are left
// unchanged. The returned State must be restored on every exit path.
func MakeRaw(input *os.File) (*State, error) {
	if input == nil {
		return nil, fmt.Errorf("terminal input must not be nil")
	}

	state := &State{fd: int(input.Fd())}
	if !IsTerminal(input) {
		return state, nil
	}

	original, err := term.MakeRaw(state.fd)
	if err != nil {
		return nil, fmt.Errorf("set terminal raw mode: %w", err)
	}

	state.original = original

	return state, nil
}

// Restore restores settings changed by MakeRaw. It is safe to call repeatedly.
func (state *State) Restore() error {
	if state == nil {
		return nil
	}

	state.once.Do(func() {
		if state.original == nil {
			return
		}

		if err := term.Restore(state.fd, state.original); err != nil {
			state.err = fmt.Errorf("restore terminal: %w", err)
		}
	})

	return state.err
}
