// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	blockadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/block"
	clusteradapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/cluster"
	"github.com/siderolabs/talos/internal/app/machined/pkg/automaton/blockautomaton"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/xfs"
)

// NodeIdentityController manages runtime.Identity caching identity in the STATE.
type NodeIdentityController struct {
	stateMachine blockautomaton.VolumeMounterAutomaton
}

// Name implements controller.Controller interface.
func (ctrl *NodeIdentityController) Name() string {
	return "cluster.NodeIdentityController"
}

// Inputs implements controller.Controller interface.
func (ctrl *NodeIdentityController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: resources.InMemoryNamespace,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
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
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *NodeIdentityController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if ctrl.stateMachine == nil {
			ctrl.stateMachine = blockautomaton.NewVolumeMounter(
				ctrl.Name(),
				constants.StatePartitionLabel,
				ctrl.establishNodeIdentity,
				blockautomaton.WithDetached(true),
			)
		}

		if err := ctrl.stateMachine.Run(ctx, r, logger); err != nil {
			return fmt.Errorf("error running volume mounter machine: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *NodeIdentityController) establishNodeIdentity(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, mountStatus *block.VolumeMountStatus) error {
	return blockadapter.VolumeMountStatus(mountStatus).WithRoot(logger, func(root xfs.Root) error {
		var localIdentity cluster.IdentitySpec

		if err := controllers.LoadOrNewFromFile(root, constants.NodeIdentityFilename, &localIdentity, func(v *cluster.IdentitySpec) error {
			return clusteradapter.IdentitySpec(v).Generate()
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
				r.TypedSpec().SelinuxLabel = constants.EtcSelinuxLabel

				return err
			}); err != nil {
			return fmt.Errorf("error modifying machine-id: %w", err)
		}

		logger.Info("node identity established", zap.String("node_id", localIdentity.NodeID))

		return nil
	})
}
