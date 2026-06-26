// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions

import (
	"context"
	"fmt"
)

// OK is returned by the String method of the passed Condition.
const OK = "OK"

// State is the observable lifecycle state of a Condition.
type State int

const (
	// StateRunning means the condition is not yet done; the last poll may have errored transiently.
	StateRunning State = iota
	// StateSucceeded means the condition's assertion returned nil.
	StateSucceeded
	// StateSkipped means the condition's assertion returned ErrSkipAssertion.
	StateSkipped
	// StateFailed means Wait returned non-nil; the condition will not be retried.
	StateFailed
)

// Condition is a object which Wait()s for some condition to become true.
//
// Condition can describe itself via String() method.
type Condition interface {
	fmt.Stringer
	Wait(ctx context.Context) error
}

// Stateful is an optional Condition extension exposing typed progress state,
// so consumers render status without parsing String().
type Stateful interface {
	Condition
	// State returns the current state and the last non-fatal poll error
	// (nil unless running after a transient error).
	State() (State, error)
}

// StatusLine returns the human-readable status line: the condition's description
// decorated with its current state (": ...", ": OK", ": SKIP", or ": <error>").
// Conditions that don't implement Stateful are returned unchanged.
func StatusLine(c Condition) string {
	s, ok := c.(Stateful)
	if !ok {
		return c.String()
	}

	state, lastErr := s.State()

	switch state {
	case StateSucceeded:
		return c.String() + ": " + OK
	case StateSkipped:
		return c.String() + ": " + ErrSkipAssertion.Error()
	case StateFailed:
		if lastErr != nil {
			return c.String() + ": " + lastErr.Error()
		}

		return c.String() + ": failed"
	case StateRunning:
		if lastErr != nil {
			return c.String() + ": " + lastErr.Error()
		}

		return c.String() + ": ..."
	}

	return c.String()
}
