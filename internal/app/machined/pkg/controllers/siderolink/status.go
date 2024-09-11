// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/siderolink"
)

// DefaultStatusUpdateInterval is the default interval between status updates.
const DefaultStatusUpdateInterval = 30 * time.Second

// StatusController reports siderolink status.
type StatusController struct {
	// WGClientFunc is a function that returns a WireguardClient.
	//
	// When nil, it defaults to an actual Wireguard client.
	WGClientFunc func() (WireguardClient, error)

	// Interval is the time between peer status checks.
	//
	// When zero, it defaults to DefaultStatusUpdateInterval.
	Interval time.Duration
}

// Name implements controller.Controller interface.
func (ctrl *StatusController) Name() string {
	return "siderolink.StatusController"
}

// Inputs implements controller.Controller interface.
func (ctrl *StatusController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      siderolink.ConfigType,
			ID:        optional.Some(siderolink.ConfigID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *StatusController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: siderolink.StatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *StatusController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	interval := ctrl.Interval
	if interval == 0 {
		interval = DefaultStatusUpdateInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	wgClientFunc := ctrl.WGClientFunc
	if wgClientFunc == nil {
		wgClientFunc = func() (WireguardClient, error) {
			return wgctrl.New()
		}
	}

	wgClient, err := wgClientFunc()
	if err != nil {
		return fmt.Errorf("failed to create wireguard client: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-ticker.C:
		}

		r.StartTrackingOutputs()

		if err = ctrl.reconcileStatus(ctx, r, wgClient); err != nil {
			return err
		}

		if err = safe.CleanupOutputs[*siderolink.Status](ctx, r); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *StatusController) reconcileStatus(ctx context.Context, r controller.Runtime, wgClient WireguardClient) (err error) {
	cfg, err := safe.ReaderGetByID[*siderolink.Config](ctx, r, siderolink.ConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return err
	}

	if cfg.TypedSpec().APIEndpoint == "" {
		return nil
	}

	host, _, err := net.SplitHostPort(cfg.TypedSpec().Host)
	if err != nil {
		host = cfg.TypedSpec().Host
	}

	down, err := peerDown(wgClient)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}

		down = true // wireguard device does not exist, we mark it as down
	}

	if err = safe.WriterModify(ctx, r, siderolink.NewStatus(), func(status *siderolink.Status) error {
		status.TypedSpec().Host = host
		status.TypedSpec().Connected = !down

		return nil
	}); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}
