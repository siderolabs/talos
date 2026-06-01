// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package automaton implements the controller-specific state automaton (state machine).
package automaton

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
)

// ControllerStateFunc is a function that implements a state in the controller state automaton.
//
// Each state in the automaton is implemented by a function which returns the next state and an error.
// If the error returned is tagged with Continue, the state automaton returns nil error keeping the state,
// this should be used to keep progressing on next controller reconcile loop.
// If the state returns an error, the state automaton returns the error and pauses the automaton.ControllerStateFunc
// If the returned next state is nil, the state automaton terminates and always returns nil.
type ControllerStateFunc[T any] func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, v T) (ControllerStateFunc[T], error)

// ControllerAutomaton is a state automaton that is used in a controller context.
//
// Type T holds a context value that is passed to each state function.
type ControllerAutomaton[T any] struct {
	state ControllerStateFunc[T]
	value T
}

// NewControllerAutomaton creates a new controller automaton with the specified initialState and value.
func NewControllerAutomaton[T any](initialState ControllerStateFunc[T], v T) *ControllerAutomaton[T] {
	return &ControllerAutomaton[T]{
		state: initialState,
		value: v,
	}
}

// Continue is an error tag that indicates that the state automaton should return to the controller with nil error and keep the state.
type Continue struct{}

// RunOptions is a struct that holds options for the Run function.
type RunOptions struct {
	AfterFunc func() error
}

// RunOption is a function that configures the RunOptions.
type RunOption func(*RunOptions)

// WithAfterFunc sets the AfterFunc option.
func WithAfterFunc(afterFunc func() error) RunOption {
	return func(options *RunOptions) {
		options.AfterFunc = afterFunc
	}
}

// Run is the entrypoint to the state automaton.
//
// Run is supposed to be called from the controller's reconcile loop.
//
// If Run returns an error, the controller should propagate the error back, and on nil error,
// the controller should wait for the next event.
func (automaton *ControllerAutomaton[T]) Run(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, options ...RunOption) error {
	opts := &RunOptions{}
	for _, opt := range options {
		opt(opts)
	}

	for {
		if automaton.state == nil {
			if opts.AfterFunc != nil {
				return opts.AfterFunc()
			}

			return nil
		}

		nextState, err := automaton.state(ctx, r, logger, automaton.value)
		if err != nil {
			if xerrors.TagIs[Continue](err) {
				return nil
			}

			return err
		}

		automaton.state = nextState
	}
}
