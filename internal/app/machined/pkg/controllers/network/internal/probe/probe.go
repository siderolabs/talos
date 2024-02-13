// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package probe contains implementation of the network probe runners.
package probe

import (
	"context"
	"errors"
	"net"
	"sync"
	"syscall"

	"github.com/benbjohnson/clock"
	"github.com/siderolabs/gen/channel"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// Runner describes a state of running probe.
type Runner struct {
	ID    string
	Spec  network.ProbeSpecSpec
	Clock clock.Clock

	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Notification of a runner status.
type Notification struct {
	ID     string
	Status network.ProbeStatusSpec
}

// Start a runner with a given context.
func (runner *Runner) Start(ctx context.Context, notifyCh chan<- Notification, logger *zap.Logger) {
	runner.wg.Add(1)

	ctx, runner.cancel = context.WithCancel(ctx)

	go func() {
		defer runner.wg.Done()

		runner.run(ctx, notifyCh, logger)
	}()
}

// Stop a runner.
func (runner *Runner) Stop() {
	runner.cancel()

	runner.wg.Wait()
}

// run a probe.
//
//nolint:gocyclo
func (runner *Runner) run(ctx context.Context, notifyCh chan<- Notification, logger *zap.Logger) {
	logger = logger.With(zap.String("probe", runner.ID))

	if runner.Clock == nil {
		runner.Clock = clock.New()
	}

	ticker := runner.Clock.Ticker(runner.Spec.Interval)
	defer ticker.Stop()

	consecutiveFailures := 0
	firstIteration := true

	for {
		if !firstIteration {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		} else {
			firstIteration = false
		}

		err := runner.probe(ctx)
		if err == nil {
			if consecutiveFailures > 0 {
				logger.Info("probe succeeded")
			}

			consecutiveFailures = 0

			if !channel.SendWithContext(ctx, notifyCh, Notification{
				ID: runner.ID,
				Status: network.ProbeStatusSpec{
					Success: true,
				},
			}) {
				return
			}

			continue
		}

		if consecutiveFailures == runner.Spec.FailureThreshold {
			logger.Error("probe failed", zap.Error(err))
		}

		consecutiveFailures++

		if consecutiveFailures < runner.Spec.FailureThreshold {
			continue
		}

		if !channel.SendWithContext(ctx, notifyCh, Notification{
			ID: runner.ID,
			Status: network.ProbeStatusSpec{
				Success:   false,
				LastError: err.Error(),
			},
		}) {
			return
		}
	}
}

// probe runs a probe.
func (runner *Runner) probe(ctx context.Context) error {
	var zeroTCP network.TCPProbeSpec

	switch {
	case runner.Spec.TCP != zeroTCP:
		return runner.probeTCP(ctx)
	default:
		return errors.New("no probe type specified")
	}
}

// probeTCP runs a TCP probe.
func (runner *Runner) probeTCP(ctx context.Context) error {
	dialer := &net.Dialer{
		// The dialer reduces the TIME-WAIT period to 1 seconds instead of the OS default of 60 seconds.
		Control: func(network, address string, c syscall.RawConn) error {
			return c.Control(func(fd uintptr) {
				syscall.SetsockoptLinger(int(fd), syscall.SOL_SOCKET, syscall.SO_LINGER, &syscall.Linger{Onoff: 1, Linger: 1}) //nolint: errcheck
			})
		},
	}

	ctx, cancel := context.WithTimeout(ctx, runner.Spec.TCP.Timeout)
	defer cancel()

	conn, err := dialer.DialContext(ctx, "tcp", runner.Spec.TCP.Endpoint)
	if err != nil {
		return err
	}

	return conn.Close()
}
