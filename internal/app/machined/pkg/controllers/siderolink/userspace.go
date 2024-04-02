// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"context"
	"fmt"
	"net/netip"
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

	relayRetryTimer := time.NewTimer(0)

	safeReset(relayRetryTimer, 0)

	var (
		tunnelDevice tunnelDeviceProps
		tunnelRelay  tunnelProps
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
		case <-relayRetryTimer.C:
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

		if tunnelRelay.relay.IsClosed() ||
			tunnelRelay.dstHost != ep.Host ||
			tunnelRelay.ourAddrPort != res.TypedSpec().NodeAddress {
			// Reset timer because we are going to start tunnel anyway
			safeReset(relayRetryTimer, 0)

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

				safeReset(relayRetryTimer, retryIn)

				return nil
			})
		}
	}
}

func safeReset(timer *time.Timer, in time.Duration) {
	if !timer.Stop() {
		<-timer.C
	}

	if in == 0 {
		return
	}

	timer.Reset(in)
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
