// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubespan

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"
	"go4.org/netipx"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/kubespan"
)

// PeerSpecController watches cluster.Affiliates updates PeerSpec.
type PeerSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *PeerSpecController) Name() string {
	return "kubespan.PeerSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *PeerSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      kubespan.ConfigType,
			ID:        optional.Some(kubespan.ConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.AffiliateType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cluster.NamespaceName,
			Type:      cluster.IdentityType,
			ID:        optional.Some(cluster.LocalIdentity),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *PeerSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: kubespan.PeerSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *PeerSpecController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*kubespan.Config](ctx, r, kubespan.ConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting kubespan configuration: %w", err)
		}

		localIdentity, err := safe.ReaderGetByID[*cluster.Identity](ctx, r, cluster.LocalIdentity)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting first MAC address: %w", err)
		}

		affiliates, err := safe.ReaderListAll[*cluster.Affiliate](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing cluster affiliates: %w", err)
		}

		touchedIDs := map[resource.ID]struct{}{}

		if cfg != nil && localIdentity != nil && cfg.TypedSpec().Enabled {
			localAffiliateID := localIdentity.TypedSpec().NodeID

			peerIPSets := make(map[string]*netipx.IPSet, affiliates.Len())

		affiliateLoop:
			for affiliate := range affiliates.All() {
				if affiliate.Metadata().ID() == localAffiliateID {
					// skip local affiliate, it's not a peer
					continue
				}

				spec := affiliate.TypedSpec()

				if spec.KubeSpan.PublicKey == "" {
					// no kubespan information, skip it
					continue
				}

				var builder netipx.IPSetBuilder

				for _, ipPrefix := range spec.KubeSpan.AdditionalAddresses {
					builder.AddPrefix(ipPrefix)
				}

				for _, ip := range spec.Addresses {
					builder.Add(ip)
				}

				builder.Add(spec.KubeSpan.Address)

				var ipSet *netipx.IPSet

				ipSet, err = builder.IPSet()
				if err != nil {
					logger.Warn("failed building list of IP ranges for the peer", zap.String("ignored_peer", spec.KubeSpan.PublicKey), zap.String("label", spec.Nodename), zap.Error(err))

					continue
				}

				for otherPublicKey, otherIPSet := range peerIPSets {
					if otherIPSet.Overlaps(ipSet) {
						logger.Warn("peer address overlap", zap.String("this_peer", spec.KubeSpan.PublicKey), zap.String("other_peer", otherPublicKey),
							zap.Strings("this_ips", dumpSet(ipSet)), zap.Strings("other_ips", dumpSet(otherIPSet)))

						// exclude overlapping IPs from the ipSet
						var bldr netipx.IPSetBuilder

						// ipSet = ipSet & ~otherIPSet
						bldr.AddSet(otherIPSet)
						bldr.Complement()
						bldr.Intersect(ipSet)

						ipSet, err = bldr.IPSet()
						if err != nil {
							logger.Warn("failed building list of IP ranges for the peer", zap.String("ignored_peer", spec.KubeSpan.PublicKey), zap.String("label", spec.Nodename), zap.Error(err))

							continue affiliateLoop
						}

						if len(ipSet.Ranges()) == 0 {
							logger.Warn("conflict resolution removed all ranges", zap.String("this_peer", spec.KubeSpan.PublicKey), zap.String("other_peer", otherPublicKey))
						}
					}
				}

				peerIPSets[spec.KubeSpan.PublicKey] = ipSet

				if err = safe.WriterModify(ctx, r, kubespan.NewPeerSpec(kubespan.NamespaceName, spec.KubeSpan.PublicKey), func(res *kubespan.PeerSpec) error {
					*res.TypedSpec() = kubespan.PeerSpecSpec{
						Address:    spec.KubeSpan.Address,
						AllowedIPs: ipSet.Prefixes(),
						Endpoints:  slices.Clone(spec.KubeSpan.Endpoints),
						Label:      spec.Nodename,
					}

					return nil
				}); err != nil {
					return err
				}

				touchedIDs[spec.KubeSpan.PublicKey] = struct{}{}
			}
		}

		// list keys for cleanup
		list, err := safe.ReaderListAll[*kubespan.PeerSpec](ctx, r)
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

// dumpSet converts IPSet to a form suitable for logging.
func dumpSet(set *netipx.IPSet) []string {
	return xslices.Map(set.Ranges(), netipx.IPRange.String)
}
