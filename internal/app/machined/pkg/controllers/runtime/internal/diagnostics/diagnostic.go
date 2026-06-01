// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package diagnostics provides Talos diagnostics specific checks.
package diagnostics

import (
	"context"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Check defines a function that checks for a specific issue.
//
// If the check produces a warning, it should return a non-nil warning and nil error.
// If the check produces an error, the error will be logged, and other checks will proceed running.
type Check func(ctx context.Context, r controller.Reader, logger *zap.Logger) (*runtime.DiagnosticSpec, error)

// CheckDescription combines a check with a semantic ID.
type CheckDescription struct {
	// Semantic ID is used to identify the check and help message.
	ID string

	// Hysteresis time to wait before announcing the warning after the first appearance.
	Hysteresis time.Duration

	// Check function to run.
	Check Check
}

// Checks returns a list of checks to be run by the diagnostics engine.
func Checks() []CheckDescription {
	return []CheckDescription{
		{
			ID:         "address-overlap",
			Hysteresis: 30 * time.Second,
			Check:      AddressOverlapCheck,
		},
		{
			ID:         "kubelet-csr",
			Hysteresis: 30 * time.Second,
			Check:      KubeletCSRNotApprovedCheck,
		},
	}
}
