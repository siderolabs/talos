// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

// MemberController converts Affiliates which have Nodename set into Members.
type MemberController struct{}

// Name implements controller.Controller interface.
func (ctrl *MemberController) Name() string {
	return "cluster.MemberController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MemberController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.AffiliateType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MemberController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: cluster.MemberType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *MemberController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		affiliates, err := safe.ReaderListAll[*cluster.Affiliate](ctx, r)
		if err != nil {
			return errors.New("error listing affiliates")
		}

		touchedIDs := make(map[resource.ID]struct{})

		for affiliate := range affiliates.All() {
			affiliateSpec := affiliate.TypedSpec()
			if affiliateSpec.Nodename == "" {
				// not a cluster member
				continue
			}

			if err = safe.WriterModify(ctx, r, cluster.NewMember(cluster.NamespaceName, affiliateSpec.Nodename), func(res *cluster.Member) error {
				spec := res.TypedSpec()

				spec.Addresses = slices.Clone(affiliateSpec.Addresses)
				spec.Hostname = affiliateSpec.Hostname
				spec.MachineType = affiliateSpec.MachineType
				spec.OperatingSystem = affiliateSpec.OperatingSystem
				spec.NodeID = affiliateSpec.NodeID
				spec.ControlPlane = affiliateSpec.ControlPlane

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[affiliateSpec.Nodename] = struct{}{}
		}

		// list keys for cleanup
		list, err := safe.ReaderListAll[*cluster.Member](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for res := range list.All() {
			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}
