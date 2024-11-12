// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// HostnameSpecController applies network.HostnameSpec to the actual interfaces.
type HostnameSpecController struct {
	V1Alpha1Mode v1alpha1runtime.Mode
}

// Name implements controller.Controller interface.
func (ctrl *HostnameSpecController) Name() string {
	return "network.HostnameSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *HostnameSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: network.NamespaceName,
			Type:      network.HostnameSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *HostnameSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.HostnameStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *HostnameSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// list source network configuration resources
		list, err := r.List(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing source network addresses: %w", err)
		}

		// add finalizers for all live resources
		for _, res := range list.Items {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		// loop over specs and sync to statuses
		for _, res := range list.Items {
			spec := res.(*network.HostnameSpec) //nolint:forcetypeassert

			switch spec.Metadata().Phase() {
			case resource.PhaseTearingDown:
				if err = r.Destroy(ctx, resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, spec.Metadata().ID(), resource.VersionUndefined)); err != nil && !state.IsNotFoundError(err) {
					return fmt.Errorf("error destroying status: %w", err)
				}

				if err = r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer: %w", err)
				}
			case resource.PhaseRunning:
				if err = safe.WriterModify(ctx, r, network.NewHostnameStatus(network.NamespaceName, spec.Metadata().ID()), func(status *network.HostnameStatus) error {
					status.TypedSpec().Hostname = spec.TypedSpec().Hostname
					status.TypedSpec().Domainname = spec.TypedSpec().Domainname

					return nil
				}); err != nil {
					return fmt.Errorf("error modifying status: %w", err)
				}

				// apply hostname unless running in container mode
				if ctrl.V1Alpha1Mode != v1alpha1runtime.ModeContainer {
					logger.Info("setting hostname", zap.String("hostname", spec.TypedSpec().Hostname), zap.String("domainname", spec.TypedSpec().Domainname))

					if err = unix.Sethostname([]byte(spec.TypedSpec().Hostname)); err != nil {
						return fmt.Errorf("error setting hostname: %w", err)
					}

					if err = unix.Setdomainname([]byte(spec.TypedSpec().Domainname)); err != nil {
						return fmt.Errorf("error setting domainname: %w", err)
					}
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
