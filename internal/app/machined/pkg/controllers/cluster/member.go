// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster

import (
	"context"
	"fmt"
	"net/netip"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
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
func (ctrl *MemberController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		affiliates, err := r.List(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.AffiliateType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing affiliates")
		}

		touchedIDs := make(map[resource.ID]struct{})

		for _, affiliate := range affiliates.Items {
			affiliateSpec := affiliate.(*cluster.Affiliate).TypedSpec()
			if affiliateSpec.Nodename == "" {
				// not a cluster member
				continue
			}

			if err = r.Modify(ctx, cluster.NewMember(cluster.NamespaceName, affiliateSpec.Nodename), func(res resource.Resource) error {
				spec := res.(*cluster.Member).TypedSpec()

				spec.Addresses = append([]netip.Addr(nil), affiliateSpec.Addresses...)
				spec.Hostname = affiliateSpec.Hostname
				spec.MachineType = affiliateSpec.MachineType
				spec.OperatingSystem = affiliateSpec.OperatingSystem
				spec.NodeID = affiliateSpec.NodeID

				return nil
			}); err != nil {
				return err
			}

			touchedIDs[affiliateSpec.Nodename] = struct{}{}
		}

		// list keys for cleanup
		list, err := r.List(ctx, resource.NewMetadata(cluster.NamespaceName, cluster.MemberType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if res.Metadata().Owner() != ctrl.Name() {
				continue
			}

			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}
	}
}
