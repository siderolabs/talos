// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/resources/network"
)

// PlatformConfigController manages network.HostnameSpec based on machine configuration, kernel cmdline.
type PlatformConfigController struct {
	V1alpha1Platform v1alpha1runtime.Platform
}

// Name implements controller.Controller interface.
func (ctrl *PlatformConfigController) Name() string {
	return "network.PlatformConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PlatformConfigController) Inputs() []controller.Input {
	return nil
}

// Outputs implements controller.Controller interface.
func (ctrl *PlatformConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: network.HostnameSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *PlatformConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	select {
	case <-ctx.Done():
		return nil
	case <-r.EventCh():
	}

	if ctrl.V1alpha1Platform == nil {
		// no platform, no work to be done
		return nil
	}

	// platform is fetched only once (but controller might fail and restart if fetching platform fails)
	hostname, err := ctrl.V1alpha1Platform.Hostname(ctx)
	if err != nil {
		return fmt.Errorf("error getting hostname: %w", err)
	}

	if len(hostname) == 0 {
		return nil
	}

	id := network.LayeredID(network.ConfigPlatform, network.HostnameID)

	return r.Modify(
		ctx,
		network.NewHostnameSpec(network.ConfigNamespaceName, id),
		func(r resource.Resource) error {
			r.(*network.HostnameSpec).TypedSpec().ConfigLayer = network.ConfigPlatform

			return r.(*network.HostnameSpec).TypedSpec().ParseFQDN(string(hostname))
		},
	)
}
