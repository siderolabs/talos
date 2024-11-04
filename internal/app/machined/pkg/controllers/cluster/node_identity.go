// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	clusteradapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers"
	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// NodeIdentityController manages runtime.Identity caching identity in the STATE.
type NodeIdentityController struct {
	V1Alpha1Mode runtime.Mode
	StatePath    string

	identityEstablished bool
}

// Name implements controller.Controller interface.
func (ctrl *NodeIdentityController) Name() string {
	return "cluster.NodeIdentityController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeIdentityController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      runtimeres.MountStatusType,
			ID:        optional.Some(constants.StatePartitionLabel),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *NodeIdentityController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cluster.IdentityType,
			Kind: controller.OutputShared,
		},
		{
			Type: files.EtcFileSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodeIdentityController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.StatePath == "" {
		ctrl.StatePath = constants.StateMountPoint
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if _, err := r.Get(ctx, resource.NewMetadata(v1alpha1.NamespaceName, runtimeres.MountStatusType, constants.StatePartitionLabel, resource.VersionUndefined)); err != nil {
			if state.IsNotFoundError(err) {
				// in container mode STATE is always mounted
				if ctrl.V1Alpha1Mode != runtime.ModeContainer {
					// wait for the STATE to be mounted
					continue
				}
			} else {
				return fmt.Errorf("error reading mount status: %w", err)
			}
		}

		var localIdentity cluster.IdentitySpec

		if err := controllers.LoadOrNewFromFile(filepath.Join(ctrl.StatePath, constants.NodeIdentityFilename), &localIdentity, func(v any) error {
			return clusteradapter.IdentitySpec(v.(*cluster.IdentitySpec)).Generate()
		}); err != nil {
			return fmt.Errorf("error caching node identity: %w", err)
		}

		if err := safe.WriterModify(ctx, r, cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity), func(r *cluster.Identity) error {
			*r.TypedSpec() = localIdentity

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		// generate `/etc/machine-id` from node identity
		if err := safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, "machine-id"),
			func(r *files.EtcFileSpec) error {
				var err error

				r.TypedSpec().Contents, err = clusteradapter.IdentitySpec(&localIdentity).ConvertMachineID()
				r.TypedSpec().Mode = 0o444

				return err
			}); err != nil {
			return fmt.Errorf("error modifying resolv.conf: %w", err)
		}

		if !ctrl.identityEstablished {
			logger.Info("node identity established", zap.String("node_id", localIdentity.NodeID))

			ctrl.identityEstablished = true
		}

		r.ResetRestartBackoff()
	}
}
