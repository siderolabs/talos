// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	pkgetcd "github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// MemberController updates information about the local etcd member.
type MemberController struct {
	GetLocalMemberIDFunc func(ctx context.Context) (uint64, error)
}

// Name implements controller.Controller interface.
func (ctrl *MemberController) Name() string {
	return "etcd.MemberController"
}

const etcdServiceID = "etcd"

// Inputs implements controller.Controller interface.
func (ctrl *MemberController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(etcdServiceID),
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MemberController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: etcd.MemberType,
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

		m := etcd.NewMember(etcd.NamespaceName, etcd.LocalMemberID)

		etcdService, err := safe.ReaderGet[*v1alpha1.Service](ctx, r, v1alpha1.NewService(etcdServiceID).Metadata())
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting etcd service resource: %w", err)
		}

		updateMemberID := etcdService != nil && etcdService.Metadata().Phase() == resource.PhaseRunning && etcdService.TypedSpec().Healthy

		if updateMemberID {
			var memberID uint64

			memberID, err = ctrl.getLocalMemberID(ctx)
			if err != nil {
				return fmt.Errorf("error getting etcd local member ID: %w", err)
			}

			if err = safe.WriterModify(ctx, r, m, func(status *etcd.Member) error {
				status.TypedSpec().MemberID = etcd.FormatMemberID(memberID)

				return nil
			}); err != nil {
				return fmt.Errorf("error updating etcd member resource: %w", err)
			}
		} else {
			if err = r.Destroy(ctx, m.Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error destroying etcd member resource: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}

// getLocalMemberID gets the etcd member ID of the local node.
func (ctrl *MemberController) getLocalMemberID(ctx context.Context) (uint64, error) {
	if ctrl.GetLocalMemberIDFunc != nil {
		return ctrl.GetLocalMemberIDFunc(ctx)
	}

	client, err := pkgetcd.NewLocalClient(ctx)
	if err != nil {
		return 0, err
	}

	defer client.Close() //nolint:errcheck

	return client.GetMemberID(ctx)
}
