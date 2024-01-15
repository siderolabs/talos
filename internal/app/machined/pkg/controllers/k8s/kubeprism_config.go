// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"context"
	"strconv"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/controller/generic/transform"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubePrismConfigController creates config for KubePrism.
type KubePrismConfigController = transform.Controller[*config.MachineConfig, *k8s.KubePrismConfig]

// NewKubePrismConfigController instanciates the controller.
func NewKubePrismConfigController() *KubePrismConfigController {
	return transform.NewController(
		transform.Settings[*config.MachineConfig, *k8s.KubePrismConfig]{
			Name: "k8s.KubePrismConfigController",
			MapMetadataOptionalFunc: func(cfg *config.MachineConfig) optional.Optional[*k8s.KubePrismConfig] {
				if cfg.Metadata().ID() != config.V1Alpha1ID {
					return optional.None[*k8s.KubePrismConfig]()
				}

				if cfg.Config().Machine() == nil {
					return optional.None[*k8s.KubePrismConfig]()
				}

				if !cfg.Config().Machine().Features().KubePrism().Enabled() {
					return optional.None[*k8s.KubePrismConfig]()
				}

				return optional.Some(k8s.NewKubePrismConfig(k8s.NamespaceName, k8s.KubePrismConfigID))
			},
			TransformFunc: func(ctx context.Context, r controller.Reader, logger *zap.Logger, cfg *config.MachineConfig, res *k8s.KubePrismConfig) error {
				endpt, err := safe.ReaderGetByID[*k8s.KubePrismEndpoints](ctx, r, k8s.KubePrismEndpointsID)
				if err != nil {
					if state.IsNotFoundError(err) {
						return xerrors.NewTaggedf[transform.SkipReconcileTag]("KubePrism endpoints resource not found; not creating KubePrism config")
					}

					return err
				}

				spec := res.TypedSpec()
				spec.Endpoints = endpt.TypedSpec().Endpoints
				spec.Host = "127.0.0.1"
				spec.Port = cfg.Config().Machine().Features().KubePrism().Port()

				return nil
			},
		},
		transform.WithExtraInputs(
			safe.Input[*k8s.KubePrismEndpoints](controller.InputWeak),
		),
	)
}

func toPort(port string) uint32 {
	if port == "" {
		return 443
	}

	p, err := strconv.ParseUint(port, 10, 32)
	if err != nil {
		return 443
	}

	return uint32(p)
}
