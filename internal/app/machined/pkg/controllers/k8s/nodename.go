// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"fmt"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/go-pointer"
	"go.uber.org/zap"

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
			ID:        pointer.To(config.V1Alpha1ID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameStatusType,
			ID:        pointer.To(network.HostnameID),
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
func (ctrl *NodenameController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.V1Alpha1ID)
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

		hostnameResource, err := r.Get(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return err
		}

		hostnameStatus := hostnameResource.(*network.HostnameStatus).TypedSpec()

		if err = r.Modify(
			ctx,
			k8s.NewNodename(k8s.NamespaceName, k8s.NodenameID),
			func(r resource.Resource) error {
				nodename := r.(*k8s.Nodename) //nolint:errcheck,forcetypeassert

				if cfgProvider.Machine().Kubelet().RegisterWithFQDN() {
					nodename.TypedSpec().Nodename = strings.ToLower(hostnameStatus.FQDN())
				} else {
					nodename.TypedSpec().Nodename = strings.ToLower(hostnameStatus.Hostname)
				}

				nodename.TypedSpec().HostnameVersion = hostnameResource.Metadata().Version().String()
				nodename.TypedSpec().SkipNodeRegistration = cfgProvider.Machine().Kubelet().SkipNodeRegistration()

				return nil
			},
		); err != nil {
			return fmt.Errorf("error modifying nodename resource: %w", err)
		}

		r.ResetRestartBackoff()
	}
}
