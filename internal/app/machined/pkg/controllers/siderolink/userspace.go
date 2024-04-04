// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"context"
	"fmt"
	"net/netip"
	"sync"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/siderolink/pkg/wgtunnel"
	"github.com/siderolabs/siderolink/pkg/wgtunnel/wgbind"
	"github.com/siderolabs/siderolink/pkg/wgtunnel/wggrpc"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/talos/internal/pkg/ctxutil"
	"github.com/siderolabs/talos/internal/pkg/endpoint"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

// UserspaceWireguardController imlements a controller that manages a Wireguard over GRPC tunnel in userspace.
type UserspaceWireguardController struct {
	RelayRetryTimeout time.Duration
	DebugDataStream   bool
}

// Name implements controller.Controller interface.
func (ctrl *UserspaceWireguardController) Name() string {
	return "siderolink.UserspaceWireguardController"
}

// Inputs implements controller.Controller interface.
func (ctrl *UserspaceWireguardController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      siderolink.TunnelType,
			ID:        optional.Some(siderolink.TunnelID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *UserspaceWireguardController) Outputs() []controller.Output {
	return []controller.Output{}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *UserspaceWireguardController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	eg, ctx := errgroup.WithContext(ctx)

	var (
		relayRetryTimer resettableTimer
		tunnelDevice    tunnelDeviceProps
		tunnelRelay     tunnelProps
	)

	defer func() {
		tunnelRelay.relay.Close()
		tunnelDevice.device.Close()
	}()

	const (
		// maxPendingServerMessages is the maximum number of messages that can be pending in the queue before blocking.
		maxPendingServerMessages = 100
		// maxPendingClientMessages is the maximum number of messages that can be pending in the ring before being overwritten.
		maxPendingClientMessages = 100
	)

	qp := wgbind.NewQueuePair(maxPendingServerMessages, maxPendingClientMessages)

	for {
		select {
		case <-ctx.Done():
			return ctxutil.Cause(ctx)
		case <-r.EventCh():
		case <-relayRetryTimer.C():
			relayRetryTimer.Clear()
		}

		res, err := safe.ReaderGetByID[*siderolink.Tunnel](ctx, r, siderolink.TunnelID)
		if err != nil {
			if state.IsNotFoundError(err) {
				tunnelRelay.relay.Close()
				tunnelDevice.device.Close()

				continue
			}

			return fmt.Errorf("failed to read link spec: %w", err)
		}

		if tunnelDevice.device.IsClosed() {
			tunnelDevice.device.Close()

			dev, err := wgtunnel.NewTunnelDevice(res.TypedSpec().LinkName, res.TypedSpec().MTU, qp, ctrl.makeLogger(logger))
			if err != nil {
				return fmt.Errorf("failed to create tunnel device: %w", err)
			}

			// Store in outer scope because modifying the same variable will lead to the data race below
			tunnelDevice = tunnelDeviceProps{device: dev, linkName: res.TypedSpec().LinkName, mtu: res.TypedSpec().MTU}

			logger.Info("wg over grpc tunnel device created", zap.String("link_name", res.TypedSpec().LinkName))

			eg.Go(func() error {
				logger.Debug("tunnel device running")
				defer logger.Debug("tunnel device exited")

				return dev.Run()
			})
		}

		ep, err := endpoint.Parse(res.TypedSpec().APIEndpoint)
		if err != nil {
			return fmt.Errorf("failed to parse siderolink API endpoint: %w", err)
		}

		dstHost := ep.Host
		ourAddrPort := res.TypedSpec().NodeAddress

		if tunnelRelay.relay.IsClosed() || tunnelRelay.dstHost != dstHost || tunnelRelay.ourAddrPort != ourAddrPort {
			// Reset timer because we are going to start tunnel anyway
			relayRetryTimer.Reset(0)

			tunnelRelay.relay.Close()

			logger.Info(
				"updating tunnel relay",
				zap.String("old_endpoint", tunnelRelay.dstHost),
				zap.Stringer("old_node_address", tunnelRelay.ourAddrPort),
				zap.String("new_endpoint", dstHost),
				zap.Stringer("new_node_address", ourAddrPort),
			)

			relay, err := wggrpc.NewRelayToHost(dstHost, ctrl.RelayRetryTimeout, qp, ourAddrPort, withTransportCredentials(ep.Insecure))
			if err != nil {
				return fmt.Errorf("failed to create tunnel relay: %w", err)
			}

			// Store in outer scope because modifying the same variable will lead to the data race below
			tunnelRelay = tunnelProps{relay: relay, dstHost: dstHost, ourAddrPort: ourAddrPort}

			eg.Go(func() error {
				logger.Debug("running tunnel relay")

				err := relay.Run(ctx, ctrl.makeLogger(logger))
				if err == nil {
					logger.Debug("tunnel relay exited gracefully",
						zap.String("endpoint", dstHost),
						zap.Stringer("node_address", ourAddrPort),
					)

					return nil
				}

				// Relay returned an error, close the relay and print the error, device should be kept running.
				relay.Close()

				const retryIn = 5 * time.Second

				logger.Error("tunnel relay failed, retrying",
					zap.Duration("timeout", retryIn),
					zap.String("endpoint", dstHost),
					zap.Stringer("node_address", ourAddrPort),
					zap.Error(err),
				)

				relayRetryTimer.Reset(retryIn)

				return nil
			})
		}
	}
}

// makeLogger ensures that we do not spam like crazy into our ring buffer loggers unless we explicitly want to.
func (ctrl *UserspaceWireguardController) makeLogger(logger *zap.Logger) *zap.Logger {
	if ctrl.DebugDataStream {
		return logger
	}

	return logger.WithOptions(zap.IncreaseLevel(zap.InfoLevel))
}

type tunnelProps struct {
	relay       *wggrpc.Relay
	dstHost     string
	ourAddrPort netip.AddrPort
}

type tunnelDeviceProps struct {
	device   *wgtunnel.TunnelDevice
	linkName string
	mtu      int
}

// resettableTimer wraps time.Timer to allow resetting the timer to any duration.
type resettableTimer struct {
	mx    sync.Mutex
	timer *time.Timer
}

// Reset resets the timer to the given duration.
//
// If the duration is zero, the timer is removed (and stopped as needed).
// If the duration is non-zero, the timer is created if it doesn't exist, or reset if it does.
func (rt *resettableTimer) Reset(delay time.Duration) {
	rt.mx.Lock()
	defer rt.mx.Unlock()

	if delay == 0 {
		if rt.timer != nil {
			if !rt.timer.Stop() {
				<-rt.timer.C
			}

			rt.timer = nil
		}
	} else {
		if rt.timer == nil {
			rt.timer = time.NewTimer(delay)
		} else {
			if !rt.timer.Stop() {
				<-rt.timer.C
			}

			rt.timer.Reset(delay)
		}
	}
}

// Clear should be called after receiving from the timer channel.
func (rt *resettableTimer) Clear() {
	rt.mx.Lock()
	defer rt.mx.Unlock()

	rt.timer = nil
}

// C returns the timer channel.
//
// If the timer was not reset to a non-zero duration, nil is returned.
func (rt *resettableTimer) C() <-chan time.Time {
	rt.mx.Lock()
	defer rt.mx.Unlock()

	if rt.timer == nil {
		return nil
	}

	return rt.timer.C
}
