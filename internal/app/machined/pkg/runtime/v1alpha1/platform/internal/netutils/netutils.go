// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package netutils provides network-related helpers for platform implementation.
package netutils

import (
	"context"
	"log"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-retry/retry"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// Wait for the network to be ready to interact with platform metadata services.
func Wait(ctx context.Context, r state.State) error {
	log.Printf("waiting for network to be ready")

	return network.NewReadyCondition(r, network.AddressReady).Wait(ctx)
}

// WaitForDevicesReady waits for devices to be ready.
func WaitForDevicesReady(ctx context.Context, r state.State) error {
	log.Printf("waiting for devices to be ready...")

	return runtime.NewDevicesStatusCondition(r).Wait(ctx)
}

// RetryFetch retries fetching from metadata service.
func RetryFetch(ctx context.Context, f func(ctx context.Context) (string, error)) (string, error) {
	var (
		userdata string
		err      error
	)

	err = retry.Exponential(
		constants.ConfigLoadTimeout,
		retry.WithUnits(time.Second),
		retry.WithJitter(time.Second),
		retry.WithErrorLogging(true),
	).RetryWithContext(
		ctx, func(ctx context.Context) error {
			userdata, err = f(ctx)

			return err
		})
	if err != nil {
		return "", err
	}

	return userdata, err
}
