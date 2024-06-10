// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"go.uber.org/zap"

	etcdcli "github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/nethelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// AdvertisedPeerController updates advertised peer list for this instance of etcd.
type AdvertisedPeerController struct{}

// Name implements controller.Controller interface.
func (ctrl *AdvertisedPeerController) Name() string {
	return "etcd.AdvertisedPeerController"
}

// Inputs implements controller.Controller interface.
func (ctrl *AdvertisedPeerController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.SpecType,
			ID:        optional.Some(etcd.SpecID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.PKIStatusType,
			ID:        optional.Some(etcd.PKIID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("etcd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *AdvertisedPeerController) Outputs() []controller.Output {
	return nil
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *AdvertisedPeerController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		etcdService, err := safe.ReaderGet[*v1alpha1.Service](ctx, r, resource.NewMetadata(v1alpha1.NamespaceName, v1alpha1.ServiceType, "etcd", resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting etcd service: %w", err)
		}

		if !(etcdService.TypedSpec().Healthy && etcdService.TypedSpec().Running) {
			continue
		}

		etcdSpec, err := safe.ReaderGet[*etcd.Spec](ctx, r, resource.NewMetadata(etcd.NamespaceName, etcd.SpecType, etcd.SpecID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting etcd spec: %w", err)
		}

		_, err = safe.ReaderGet[*etcd.PKIStatus](ctx, r, resource.NewMetadata(etcd.NamespaceName, etcd.PKIStatusType, etcd.PKIID, resource.VersionUndefined))
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error getting etcd PKI status: %w", err)
		}

		if err = ctrl.updateAdvertisedPeers(ctx, logger, etcdSpec.TypedSpec().AdvertisedAddresses); err != nil {
			return fmt.Errorf("error updating advertised peers: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *AdvertisedPeerController) updateAdvertisedPeers(ctx context.Context, logger *zap.Logger, advertisedAddresses []netip.Addr) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	client, err := etcdcli.NewLocalClient(ctx)
	if err != nil {
		return fmt.Errorf("error creating etcd client: %w", err)
	}

	defer client.Close() //nolint:errcheck

	// figure out local member ID
	resp, err := client.MemberList(ctx)
	if err != nil {
		return fmt.Errorf("error getting member list: %w", err)
	}

	localMemberID := resp.Header.MemberId

	var localMember *etcdserverpb.Member

	for _, member := range resp.Members {
		if member.ID == localMemberID {
			localMember = member

			break
		}
	}

	if localMember == nil {
		return errors.New("local member not found in member list")
	}

	newPeerURLs := xslices.Map(advertisedAddresses, func(addr netip.Addr) string {
		return fmt.Sprintf("https://%s", nethelpers.JoinHostPort(addr.String(), constants.EtcdPeerPort))
	})
	currentPeerURLs := localMember.PeerURLs

	if slices.Equal(newPeerURLs, currentPeerURLs) {
		return nil
	}

	logger.Debug("updating etcd peer URLs",
		zap.Strings("current_peer_urls", currentPeerURLs),
		zap.Strings("new_peer_urls", newPeerURLs),
		zap.Uint64("member_id", localMemberID),
	)

	_, err = client.MemberUpdate(ctx, localMemberID, newPeerURLs)
	if err != nil {
		return fmt.Errorf("error updating member: %w", err)
	}

	logger.Info("updated etcd peer URLs",
		zap.Strings("new_peer_urls", newPeerURLs),
		zap.Uint64("member_id", localMemberID),
	)

	return nil
}
