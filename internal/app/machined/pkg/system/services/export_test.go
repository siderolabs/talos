// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services

import (
	"context"

	"github.com/containerd/containerd/v2/pkg/oci"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/gen/xslices"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system/runner"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// CreateOverlayMountRequests exposes createOverlayMountRequests for tests.
func CreateOverlayMountRequests(ctx context.Context, st state.State) error {
	return createOverlayMountRequests(ctx, st)
}

// GetOCIOptions gets all OCI options from an Extension.
func (svc *Extension) GetOCIOptions() ([]oci.SpecOpts, error) {
	envVars, err := svc.parseEnvironment()
	if err != nil {
		return nil, err
	}

	return svc.getOCIOptions(envVars, svc.Spec.Container.Mounts), nil
}

// PromotionEndpoints exposes promotionEndpoints for tests.
func PromotionEndpoints(selfEndpoints, votingMemberEndpoints, discoveredEndpoints []string) []string {
	return promotionEndpoints(xslices.ToSetFunc(selfEndpoints, normalizeEtcdEndpoint), votingMemberEndpoints, discoveredEndpoints)
}

// HostProcessArgs exposes hostProcessArgs for tests.
func (svc *Extension) HostProcessArgs() (runner.Args, error) {
	return svc.hostProcessArgs(nil)
}

// ApplyExtensionServiceConfig exposes applyExtensionServiceConfig for tests.
func (svc *Extension) ApplyExtensionServiceConfig(
	spec *runtimeres.ExtensionServiceConfigSpec,
	mounts []specs.Mount,
	envVars []string,
) ([]specs.Mount, []string, error) {
	return svc.applyExtensionServiceConfig(spec, mounts, envVars)
}
