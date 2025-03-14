// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package machine implements the controller-specific state machine.
package machine

import (
	"context"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
)

// ControllerStateFunc is a function that implements a state in the controller state machine.
//
// Each state in the machine is implemented by a function which returns the next state and an error.
// If the error returned is tagged with Continue, the state machine returns nil error keeping the state,
// this should be used to keep progressing on next controller reconcile loop.
// If the state returns an error, the state machine returns the error and pauses the machine.ControllerStateFunc
// If the returned next state is nil, the state machine terminates and always returns nil.
type ControllerStateFunc[T any] func(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, v T) (ControllerStateFunc[T], error)

// ControllerMachine is a state machine that is used in a controller context.
//
// Type T holds a context value that is passed to each state function.
type ControllerMachine[T any] struct {
	state ControllerStateFunc[T]
	value T
}

// NewControllerMachine creates a new controller machine with the specified initialState and value.
func NewControllerMachine[T any](initialState ControllerStateFunc[T], v T) *ControllerMachine[T] {
	return &ControllerMachine[T]{
		state: initialState,
		value: v,
	}
}

// Continue is an error tag that indicates that the state machine should return to the controller with nil error and keep the state.
type Continue struct{}

// Run is the entrypoint to the state machine.
//
// Run is supposed to be called from the controller's reconcile loop.
//
// If Run returns an error, the controller should propagate the error back, and on nil error,
// the controller should wait for the next event.
func (machine *ControllerMachine[T]) Run(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger) error {
	for {
		if machine.state == nil {
			return nil
		}

		nextState, err := machine.state(ctx, r, logger, machine.value)
		if err != nil {
			if xerrors.TagIs[Continue](err) {
				return nil
			}

			return err
		}

		machine.state = nextState
	}
}
