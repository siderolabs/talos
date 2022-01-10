// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	clusteradapter "github.com/talos-systems/talos/internal/app/machined/pkg/adapters/cluster"
	"github.com/talos-systems/talos/internal/app/machined/pkg/controllers"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/cluster"
	"github.com/talos-systems/talos/pkg/machinery/resources/files"
	runtimeres "github.com/talos-systems/talos/pkg/machinery/resources/runtime"
	"github.com/talos-systems/talos/pkg/machinery/resources/v1alpha1"
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
			ID:        pointer.ToString(constants.StatePartitionLabel),
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

		if err := controllers.LoadOrNewFromFile(filepath.Join(ctrl.StatePath, constants.NodeIdentityFilename), &localIdentity, func(v interface{}) error {
			return clusteradapter.IdentitySpec(v.(*cluster.IdentitySpec)).Generate()
		}); err != nil {
			return fmt.Errorf("error caching node identity: %w", err)
		}

		if err := r.Modify(ctx, cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity), func(r resource.Resource) error {
			*r.(*cluster.Identity).TypedSpec() = localIdentity

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		// generate `/etc/machine-id` from node identity
		if err := r.Modify(ctx, files.NewEtcFileSpec(files.NamespaceName, "machine-id"),
			func(r resource.Resource) error {
				var err error

				r.(*files.EtcFileSpec).TypedSpec().Contents, err = clusteradapter.IdentitySpec(&localIdentity).ConvertMachineID()
				r.(*files.EtcFileSpec).TypedSpec().Mode = 0o444

				return err
			}); err != nil {
			return fmt.Errorf("error modifying resolv.conf: %w", err)
		}

		if !ctrl.identityEstablished {
			logger.Info("node identity established", zap.String("node_id", localIdentity.NodeID))

			ctrl.identityEstablished = true
		}
	}
}
