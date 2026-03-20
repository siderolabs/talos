// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package pid contains infrastructure to track service PIDs.
package pid

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/internal/pkg/miniprocfs"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Recorder is callback to update service PID storage.
type Recorder func(serviceName string, pid int32, clearEntry bool) error

// StateRecorder is an implementation of [Recorder] that updates [State] resource.
type StateRecorder struct {
	resources state.State
}

// NewStateRecorder creates a new StateRecorder.
func NewStateRecorder(resources state.State) *StateRecorder {
	return &StateRecorder{resources: resources}
}

// Record implements [Recorder] interface.
func (r *StateRecorder) Record(serviceName string, pid int32, clearEntry bool) error {
	if clearEntry {
		err := r.resources.Destroy(context.Background(), runtime.NewServicePID(serviceName).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("failed to cleanup service PID for %q: %w", serviceName, err)
		}

		return nil
	}

	// Ignoring the error here, as the PID might be already gone, but we are only interested in long-running PIDs.
	mountNamespace, _ := miniprocfs.ReadMountNamespace(pid)

	return safe.StateModify(context.Background(), r.resources, runtime.NewServicePID(serviceName), func(res *runtime.ServicePID) error {
		res.TypedSpec().PID = pid
		res.TypedSpec().MountNamespace = mountNamespace

		return nil
	})
}
