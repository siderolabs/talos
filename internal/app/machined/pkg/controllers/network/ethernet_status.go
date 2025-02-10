// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/mdlayher/ethtool"
	"github.com/siderolabs/gen/xslices"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/watch"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EthernetStatusController reports Ethernet link statuses.
type EthernetStatusController struct{}

// Name implements controller.Controller interface.
func (ctrl *EthernetStatusController) Name() string {
	return "network.EthernetStatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EthernetStatusController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *EthernetStatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.EthernetStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *EthernetStatusController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// wait for udevd to be healthy, which implies that all link renames are done
	if err := runtime.WaitForDevicesReady(ctx, r,
		[]controller.Input{
			{
				Namespace: network.NamespaceName,
				Type:      network.LinkSpecType,
				Kind:      controller.InputWeak,
			},
		},
	); err != nil {
		return err
	}

	// create watch connections to ethtool via genetlink
	// these connections are used only to join multicast groups and receive notifications on changes
	// other connections are used to send requests and receive responses, as we can't mix the notifications and request/responses
	ethtoolWatcher, err := watch.NewEthtool(watch.NewDefaultRateLimitedTrigger(ctx, r))
	if err != nil {
		logger.Warn("ethtool watcher failed to start", zap.Error(err))

		return nil
	}

	defer ethtoolWatcher.Done()

	ethClient, err := ethtool.New()
	if err != nil {
		logger.Warn("error dialing ethtool socket", zap.Error(err))

		return nil
	}

	defer ethClient.Close() //nolint:errcheck

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		r.StartTrackingOutputs()

		if err = ctrl.reconcile(ctx, r, logger, ethClient); err != nil {
			return err
		}

		if err = safe.CleanupOutputs[*network.EthernetStatus](ctx, r); err != nil {
			return err
		}
	}
}

// reconcile function runs for every reconciliation loop querying the ethtool state and updating resources.
//
//nolint:gocyclo
func (ctrl *EthernetStatusController) reconcile(
	ctx context.Context,
	r controller.Runtime,
	logger *zap.Logger,
	ethClient *ethtool.Client,
) error {
	linkInfos, err := ethClient.LinkInfos()
	if err != nil {
		return fmt.Errorf("error listing links: %w", err)
	}

	for _, linkInfo := range linkInfos {
		iface := linkInfo.Interface

		lgger := logger.With(zap.String("interface", iface.Name))

		linkState, err := ethClient.LinkState(iface)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			lgger.Warn("error getting link state", zap.Error(err))
		}

		linkMode, err := ethClient.LinkMode(iface)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			lgger.Warn("error getting link mode", zap.Error(err))
		}

		rings, err := ethClient.Rings(iface)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			lgger.Warn("error getting rings", zap.Error(err))
		}

		features, err := ethClient.Features(iface)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			lgger.Warn("error getting features", zap.Error(err))
		}

		if err := safe.WriterModify(ctx, r, network.NewEthernetStatus(network.NamespaceName, iface.Name), func(res *network.EthernetStatus) error {
			res.TypedSpec().Port = nethelpers.Port(linkInfo.Port)

			if linkMode != nil {
				res.TypedSpec().Duplex = nethelpers.Duplex(linkMode.Duplex)
				res.TypedSpec().OurModes = xslices.Map(linkMode.Ours, func(m ethtool.AdvertisedLinkMode) string { return m.Name })
				res.TypedSpec().PeerModes = xslices.Map(linkMode.Peer, func(m ethtool.AdvertisedLinkMode) string { return m.Name })
			} else {
				res.TypedSpec().Duplex = nethelpers.Duplex(0)
			}

			if linkState == nil {
				res.TypedSpec().LinkState = nil
			} else {
				res.TypedSpec().LinkState = pointer.To(linkState.Link)
			}

			if rings == nil {
				res.TypedSpec().Rings = nil
			} else {
				res.TypedSpec().Rings = &network.EthernetRingsStatus{
					RXMax:           rings.RXMax.Ptr(),
					RXMiniMax:       rings.RXMiniMax.Ptr(),
					RXJumboMax:      rings.RXJumboMax.Ptr(),
					TXMax:           rings.TXMax.Ptr(),
					TXPushBufLenMax: rings.TXPushBufLenMax.Ptr(),
					RX:              rings.RX.Ptr(),
					RXMini:          rings.RXMini.Ptr(),
					RXJumbo:         rings.RXJumbo.Ptr(),
					TX:              rings.TX.Ptr(),
					RXBufLen:        rings.RXBufLen.Ptr(),
					CQESize:         rings.CQESize.Ptr(),
					TXPush:          rings.TXPush.Ptr(),
					RXPush:          rings.RXPush.Ptr(),
					TXPushBufLen:    rings.TXPushBufLen.Ptr(),
					TCPDataSplit:    rings.TCPDataSplit.Ptr(),
				}
			}

			if features == nil {
				res.TypedSpec().Features = nil
			} else {
				res.TypedSpec().Features = xslices.Map(features, func(f ethtool.FeatureInfo) network.EthernetFeatureStatus {
					return network.EthernetFeatureStatus{
						Name:   f.Name,
						Status: f.State() + f.Suffix(),
					}
				})
			}

			return nil
		}); err != nil {
			return fmt.Errorf("error updating EthernetStatus resource: %w", err)
		}
	}

	return nil
}
