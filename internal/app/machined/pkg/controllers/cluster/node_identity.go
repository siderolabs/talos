// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/resources/cluster"
	runtimeres "github.com/talos-systems/talos/pkg/resources/runtime"
	"github.com/talos-systems/talos/pkg/resources/v1alpha1"
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
	}
}

func loadOrNewFromState(statePath, path string, empty interface{}, generate func(interface{}) error) error {
	fullPath := filepath.Join(statePath, path)

	f, err := os.OpenFile(fullPath, os.O_RDONLY, 0)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("error reading state file: %w", err)
	}

	// file doesn't exist yet, generate new value and save it
	if f == nil {
		if err = generate(empty); err != nil {
			return err
		}

		f, err = os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
		if err != nil {
			return fmt.Errorf("error creating state file: %w", err)
		}

		defer f.Close() //nolint:errcheck

		encoder := yaml.NewEncoder(f)
		if err = encoder.Encode(empty); err != nil {
			return fmt.Errorf("error marshaling: %w", err)
		}

		if err = encoder.Close(); err != nil {
			return err
		}

		return f.Close()
	}

	// read existing cached value
	defer f.Close() //nolint:errcheck

	if err = yaml.NewDecoder(f).Decode(empty); err != nil {
		return fmt.Errorf("error unmarshaling: %w", err)
	}

	if reflect.ValueOf(empty).Elem().IsZero() {
		return fmt.Errorf("value is still zero after unmarshaling")
	}

	return f.Close()
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

		if err := loadOrNewFromState(ctrl.StatePath, constants.NodeIdentityFilename, &localIdentity, func(v interface{}) error {
			return v.(*cluster.IdentitySpec).Generate()
		}); err != nil {
			return fmt.Errorf("error caching node identity: %w", err)
		}

		if err := r.Modify(ctx, cluster.NewIdentity(cluster.NamespaceName, cluster.LocalIdentity), func(r resource.Resource) error {
			*r.(*cluster.Identity).TypedSpec() = localIdentity

			return nil
		}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		if !ctrl.identityEstablished {
			logger.Info("node identity established", zap.String("node_id", localIdentity.NodeID))

			ctrl.identityEstablished = true
		}
	}
}
