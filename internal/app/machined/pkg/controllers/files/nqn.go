// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	clusteradapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/cluster"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// NQNController creates an EtcFileSpec for the NVMe Qualified Name (NQN) and HostID file.
type NQNController struct {
	V1Alpha1Mode runtimetalos.Mode
}

// Name implements controller.Controller interface.
func (ctrl *NQNController) Name() string {
	return "files.NQNController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NQNController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        optional.Some(cluster.LocalIdentity),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NQNController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.EtcFileSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NQNController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	// Skip the controller if we're running in a container.
	if ctrl.V1Alpha1Mode == runtimetalos.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		// get the local node identity
		localIdentity, err := safe.ReaderGetByID[*cluster.Identity](ctx, r, cluster.LocalIdentity)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to get machine-id etcfile status: %w", err)
		}

		machineID, err := clusteradapter.IdentitySpec(localIdentity.TypedSpec()).ConvertMachineID()
		if err != nil {
			return fmt.Errorf("failed to convert identity to machine ID: %w", err)
		}

		hostID, err := uuid.FromBytes(machineID[:16])
		if err != nil {
			return fmt.Errorf("failed to convert machine-id to UUID: %w", err)
		}

		if err := safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, "nvme/hostid"),
			func(r *files.EtcFileSpec) error {
				spec := r.TypedSpec()

				spec.Contents = []byte(hostID.String())
				spec.Mode = 0o600
				spec.SelinuxLabel = constants.EtcSelinuxLabel

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		if err := safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, "nvme/hostnqn"),
			func(r *files.EtcFileSpec) error {
				spec := r.TypedSpec()

				// Fri Nov 3 16:19:12 2017 -0700 is the date of the first commit in the talos repository.
				spec.Contents = []byte(fmt.Sprintf("nqn.2017-11.dev.talos:uuid:%s", hostID.String()))
				spec.Mode = 0o600
				spec.SelinuxLabel = constants.EtcSelinuxLabel

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}
	}
}
