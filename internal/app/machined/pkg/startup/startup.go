// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package startup provides machined startup tasks.
package startup

import (
	"context"

	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
)

// Task is a function that performs a startup task.
//
// It is supposed to call the next task in the chain.
type Task func(context.Context, *zap.Logger, runtime.Runtime, NextTaskFunc) error

// NextTaskFunc is a function which returns the next task in the chain.
type NextTaskFunc func() Task

// RunTasks runs the given tasks in order.
func RunTasks(ctx context.Context, log *zap.Logger, rt runtime.Runtime, tasks ...Task) error {
	var idx int

	nextTaskFunc := func() Task {
		idx++

		return tasks[idx]
	}

	return tasks[0](ctx, log, rt, nextTaskFunc)
}

// DefaultTasks returns the default startup tasks.
func DefaultTasks() []Task {
	return []Task{
		LogMode,
		MountPseudoLate,
		SetupSystemDirectories,
		InitVolumeLifecycle,
		MountCgroups,
		SetRLimit,
		SetEnvironmentVariables,
		CreateSystemCgroups,
	}
}
