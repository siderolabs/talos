// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lifecycle holds a finalizer on a shutdown barrier, the resource workloads are wound down
// behind before services stop.
//
// Each workload kind has its own barrier resource: a startup task creates it, and a sequencer task
// records a shutdown deadline on it, tears it down and waits for its finalizers to empty. Every
// controller which owns a workload that has to be wound down while services still run holds a
// finalizer on its barrier.
package lifecycle

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
)

// Barrier is a pointer to a barrier resource type. The pointer constraint lets a nil barrier stand
// for one which is gone: a nil pointer wrapped in an interface would not compare equal to nil.
type Barrier[T any] interface {
	*T
	meta.ResourceWithRD
}

// Read returns the barrier with the given ID, or nil if it does not exist.
//
// The startup task creates it before the controller runtime starts, so it is missing only once the
// shutdown has released and destroyed it.
func Read[T any, PT Barrier[T]](ctx context.Context, reader controller.Reader, id resource.ID) (PT, error) {
	barrier, err := safe.ReaderGetByID[PT](ctx, reader, id)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("failed to get the shutdown barrier: %w", err)
	}

	return barrier, nil
}

// ShuttingDown reports whether the node is on its way down: the barrier is tearing down, or gone.
//
// A controller seeing this winds down what it owns and starts nothing again. Treating a missing
// barrier as normal operation would start the workloads straight back up while their runtime is
// still there.
func ShuttingDown[T any, PT Barrier[T]](barrier PT) bool {
	return barrier == nil || barrier.Metadata().Phase() == resource.PhaseTearingDown
}

// Finalizers is the part of a controller writer the barrier is held with.
type Finalizers interface {
	AddFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
	RemoveFinalizer(context.Context, resource.Pointer, ...resource.Finalizer) error
}

// Reconcile holds a finalizer on the barrier on behalf of owner.
//
// The finalizer set is the payload, and the shutdown sequence blocks until it is empty. A holder
// releases its finalizer only once releasable reports that it has nothing left to wind down.
//
// Holding it is only half of the contract: a holder must also react to the barrier tearing down by
// winding down what it owns, or the shutdown sequence waits on a finalizer that is never released.
func Reconcile[T any, PT Barrier[T]](
	ctx context.Context,
	r Finalizers,
	logger *zap.Logger,
	barrier PT,
	owner string,
	releasable bool,
) error {
	if barrier == nil {
		return nil
	}

	hasFinalizer := barrier.Metadata().Finalizers().Has(owner)

	switch barrier.Metadata().Phase() {
	case resource.PhaseRunning:
		if !hasFinalizer {
			if err := r.AddFinalizer(ctx, barrier.Metadata(), owner); err != nil {
				return fmt.Errorf("failed to add shutdown barrier finalizer: %w", err)
			}

			logger.Debug("holding the shutdown barrier", zap.String("barrier", barrier.Metadata().Type()))
		}
	case resource.PhaseTearingDown:
		// Not logging the still-waiting case: it would repeat on every reconcile for the length of
		// the shutdown, and the holders already log each thing they are winding down.
		if hasFinalizer && releasable {
			if err := r.RemoveFinalizer(ctx, barrier.Metadata(), owner); err != nil {
				return fmt.Errorf("failed to remove shutdown barrier finalizer: %w", err)
			}

			logger.Info("released the shutdown barrier", zap.String("barrier", barrier.Metadata().Type()))
		}
	}

	return nil
}
