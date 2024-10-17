// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package netutils provides network-related helpers for platform implementation.
package netutils

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/cenkalti/backoff/v4"
	"github.com/cosi-project/runtime/pkg/safe"
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

// WaitInterfaces for the interfaces to be up to interact with platform metadata services.
func WaitInterfaces(ctx context.Context, r state.State) error {
	backoff := backoff.NewExponentialBackOff()
	backoff.MaxInterval = 2 * time.Second
	backoff.MaxElapsedTime = 30 * time.Second

	for ctx.Err() == nil {
		hostInterfaces, err := safe.StateListAll[*network.LinkStatus](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing host interfaces: %w", err)
		}

		numPhysical := 0

		for iface := range hostInterfaces.All() {
			if iface.TypedSpec().Physical() {
				numPhysical++
			}
		}

		if numPhysical > 0 {
			return nil
		}

		log.Printf("waiting for physical network interfaces to appear...")

		interval := backoff.NextBackOff()

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(interval):
		}
	}

	return nil
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
