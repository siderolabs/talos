// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/mdlayher/ethtool"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/value"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// EthernetSpecController reports Ethernet link statuses.
type EthernetSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *EthernetSpecController) Name() string {
	return "network.EthernetSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EthernetSpecController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *EthernetSpecController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
func (ctrl *EthernetSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// wait for udevd to be healthy, which implies that all link renames are done
	if err := runtime.WaitForDevicesReady(
		ctx, r,
		[]controller.Input{
			{
				Namespace: network.NamespaceName,
				Type:      network.EthernetSpecType,
				Kind:      controller.InputWeak,
			},
		},
	); err != nil {
		return err
	}

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

		specs, err := safe.ReaderListAll[*network.EthernetSpec](ctx, r)
		if err != nil {
			return fmt.Errorf("error reading EthernetSpec resources: %w", err)
		}

		var errs error

		for spec := range specs.All() {
			if err = ctrl.apply(ethClient, spec, logger); err != nil {
				errs = errors.Join(errs, fmt.Errorf("error configuring %q: %w", spec.Metadata().ID(), err))
			}
		}

		if errs != nil {
			return fmt.Errorf("failed to reconcile Ethernet specs: %w", errs)
		}

		r.ResetRestartBackoff()
	}
}

func optionalFromPtr[T any](ptr *T) optional.Optional[T] {
	if ptr == nil {
		return optional.None[T]()
	}

	return optional.Some(*ptr)
}

func (ctrl *EthernetSpecController) apply(
	ethClient *ethtool.Client,
	spec *network.EthernetSpec,
	logger *zap.Logger,
) error {
	iface := ethtool.Interface{
		Name: spec.Metadata().ID(),
	}

	if err := applyRings(ethClient, iface, spec.TypedSpec().Rings); err != nil {
		return fmt.Errorf("error updating rings: %w", err)
	}

	if err := applyFeatures(ethClient, iface, spec.TypedSpec().Features); err != nil {
		return fmt.Errorf("error updating features: %w", err)
	}

	if err := applyChannels(ethClient, iface, spec.TypedSpec().Channels); err != nil {
		return fmt.Errorf("error updating channels: %w", err)
	}

	if err := applyWakeOnLAN(ethClient, iface, spec.TypedSpec().WakeOnLAN); err != nil {
		return fmt.Errorf("error updating wake-on-lan: %w", err)
	}

	if err := applyFlowControl(ethClient, iface, spec.TypedSpec().FlowControl); err != nil {
		// not all drivers support flow control (e.g. virtio-net), so don't fail the whole reconcile
		if errors.Is(err, unix.EOPNOTSUPP) {
			logger.Warn("flow control is not supported by the link driver", zap.String("link", iface.Name))
		} else {
			return fmt.Errorf("error updating flow control: %w", err)
		}
	}

	return nil
}

func applyRings(ethClient *ethtool.Client, iface ethtool.Interface, ringSpec network.EthernetRingsSpec) error {
	if value.IsZero(ringSpec) {
		return nil
	}

	return ethClient.SetRings(ethtool.Rings{
		Interface:    iface,
		RX:           optionalFromPtr(ringSpec.RX),
		RXMini:       optionalFromPtr(ringSpec.RXMini),
		RXJumbo:      optionalFromPtr(ringSpec.RXJumbo),
		TX:           optionalFromPtr(ringSpec.TX),
		RXBufLen:     optionalFromPtr(ringSpec.RXBufLen),
		CQESize:      optionalFromPtr(ringSpec.CQESize),
		TXPush:       optionalFromPtr(ringSpec.TXPush),
		RXPush:       optionalFromPtr(ringSpec.RXPush),
		TXPushBufLen: optionalFromPtr(ringSpec.TXPushBufLen),
		TCPDataSplit: optionalFromPtr(ringSpec.TCPDataSplit),
	})
}

func applyFeatures(ethClient *ethtool.Client, iface ethtool.Interface, featureSpec map[string]bool) error {
	if len(featureSpec) == 0 {
		return nil
	}

	return ethClient.SetFeatures(iface, featureSpec)
}

func applyChannels(ethClient *ethtool.Client, iface ethtool.Interface, channelsSpec network.EthernetChannelsSpec) error {
	if value.IsZero(channelsSpec) {
		return nil
	}

	return ethClient.SetChannels(ethtool.Channels{
		Interface:     iface,
		RXCount:       optionalFromPtr(channelsSpec.RX),
		TXCount:       optionalFromPtr(channelsSpec.TX),
		OtherCount:    optionalFromPtr(channelsSpec.Other),
		CombinedCount: optionalFromPtr(channelsSpec.Combined),
	})
}

func applyWakeOnLAN(ethClient *ethtool.Client, iface ethtool.Interface, wolSpec []nethelpers.WOLMode) error {
	if wolSpec == nil {
		return nil
	}

	var wolModes nethelpers.WOLMode

	for _, mode := range wolSpec {
		wolModes |= mode
	}

	return ethClient.SetWakeOnLAN(ethtool.WakeOnLAN{
		Interface: iface,
		Modes:     ethtool.WOLMode(wolModes),
	})
}

func applyFlowControl(ethClient *ethtool.Client, iface ethtool.Interface, flowControlSpec network.EthernetFlowControlSpec) error {
	if value.IsZero(flowControlSpec) {
		return nil
	}

	return ethClient.SetPause(ethtool.Pause{
		Interface: iface,
		RX:        optionalFromPtr(flowControlSpec.RX),
		TX:        optionalFromPtr(flowControlSpec.TX),
		Autoneg:   optionalFromPtr(flowControlSpec.Autoneg),
	})
}
