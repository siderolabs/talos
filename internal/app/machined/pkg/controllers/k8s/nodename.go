// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/k8s/internal/nodename"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// NodenameController renders manifests based on templates and config/secrets.
type NodenameController struct{}

// Name implements controller.Controller interface.
func (ctrl *NodenameController) Name() string {
	return "k8s.NodenameController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodenameController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        optional.Some(network.HostnameID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodenameController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: k8s.NodenameType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodenameController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting config: %w", err)
		}

		cfgProvider := cfg.Config()

		if cfgProvider.Machine() == nil {
			continue
		}

		hostnameStatus, err := safe.ReaderGetByID[*network.HostnameStatus](ctx, r, network.HostnameID)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		if err = safe.WriterModify(
			ctx,
			r,
			k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID),
			func(res *k8s.Nodename) error {
				var hostname string

				if cfgProvider.Machine().Kubelet().RegisterWithFQDN() {
					hostname = hostnameStatus.TypedSpec().FQDN()
				} else {
					hostname = hostnameStatus.TypedSpec().Hostname
				}

				res.TypedSpec().Nodename, err = nodename.FromHostname(hostname)
				if err != nil {
					return err
				}

				res.TypedSpec().HostnameVersion = hostnameStatus.Metadata().Version().String()
				res.TypedSpec().SkipNodeRegistration = cfgProvider.Machine().Kubelet().SkipNodeRegistration()

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying nodename resource: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
