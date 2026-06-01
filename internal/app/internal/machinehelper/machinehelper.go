// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package machinehelper provides helper functions for machine-related information.
package machinehelper

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
)

// CheckControlplane implements the controlplane machine type check.
//
// This works for API handlers.
func CheckControlplane(ctx context.Context, resources state.State, apiName string) error {
	machineType, err := safe.StateGetByID[*config.MachineType](ctx, resources, config.MachineTypeID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return status.Errorf(codes.Unimplemented, "machine type is not set, cannot use %s API", apiName)
		}

		return fmt.Errorf("failed to get machine type: %w", err)
	}

	if !machineType.MachineType().IsControlPlane() {
		return status.Errorf(codes.Unimplemented, "%s is only available on control plane nodes", apiName)
	}

	return nil
}
