// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"log"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/app/networkd/pkg/networkd"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/runtime"
)

// ResetNetworkNetwork represents the ResetNetworkNetwork task.
type ResetNetworkNetwork struct{}

// NewResetNetworkTask initializes and returns an ResetNetworkNetwork task.
func NewResetNetworkTask() phase.Task {
	return &ResetNetworkNetwork{}
}

// TaskFunc returns the runtime function.
func (task *ResetNetworkNetwork) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	switch mode {
	case runtime.Container:
		return nil
	default:
		return task.runtime
	}
}

// nolint: gocyclo
func (task *ResetNetworkNetwork) runtime(r runtime.Runtime) (err error) {
	// Check to see if a static IP was set via kernel args;
	// if so, we'll skip the initial dhcp discovery
	if option := kernel.ProcCmdline().Get("ip").First(); option != nil {
		log.Println("skipping initial network setup, found kernel arg 'ip'")
		return nil
	}

	nwd, err := networkd.New(r.Config())
	if err != nil {
		return err
	}

	nwd.Reset()

	return nil
}
