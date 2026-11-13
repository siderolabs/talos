// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	etcdcli "github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/meta"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// MembershipSpecController gathers everything required to decide on the etcd cluster membership of this machine
// into a single resource, which is the configuration of the MembershipController.
//
// The MembershipSpec exists only if all of its inputs are available: if etcd is not configured,
// or if something is not known yet (e.g. the machine type), there is no MembershipSpec.
type MembershipSpecController struct{}

// Name implements controller.Controller interface.
func (ctrl *MembershipSpecController) Name() string {
	return "etcd.MembershipSpecController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MembershipSpecController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.SpecType,
			ID:        optional.Some(etcd.SpecID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineTypeType,
			ID:        optional.Some(config.MachineTypeID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaLoadedType,
			ID:        optional.Some(runtime.MetaLoadedID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.MetaKeyType,
			ID:        optional.Some(runtime.MetaKeyTagToID(meta.Upgrade)),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MembershipSpecController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: etcd.MembershipSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *MembershipSpecController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		membershipSpec, err := ctrl.build(ctx, r)
		if err != nil {
			return err
		}

		if membershipSpec == nil {
			if err = r.Destroy(ctx, etcd.NewMembershipSpec().Metadata()); err != nil && !state.IsNotFoundError(err) {
				return fmt.Errorf("error destroying etcd membership spec: %w", err)
			}
		} else {
			if err = safe.WriterModify(ctx, r, etcd.NewMembershipSpec(), func(res *etcd.MembershipSpec) error {
				*res.TypedSpec() = *membershipSpec

				return nil
			}); err != nil {
				return fmt.Errorf("error updating etcd membership spec: %w", err)
			}
		}

		r.ResetRestartBackoff()
	}
}

// build the membership spec, it returns nil if some of the inputs are not available.
func (ctrl *MembershipSpecController) build(ctx context.Context, r controller.Runtime) (*etcd.MembershipSpecSpec, error) {
	spec, err := safe.ReaderGetByID[*etcd.Spec](ctx, r, etcd.SpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// etcd is not configured
			return nil, nil
		}

		return nil, fmt.Errorf("error getting etcd spec: %w", err)
	}

	machineType, err := safe.ReaderGetByID[*config.MachineType](ctx, r, config.MachineTypeID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error getting machine type: %w", err)
	}

	membershipSpec := &etcd.MembershipSpecSpec{
		Name:                 spec.TypedSpec().Name,
		AdvertisedAddresses:  spec.TypedSpec().AdvertisedAddresses,
		ManualInitialCluster: etcdcli.HasExtraArg(spec.TypedSpec(), etcdcli.ArgInitialCluster),
		InitNode:             machineType.MachineType() == machine.TypeInit,
	}

	if !membershipSpec.InitNode {
		return membershipSpec, nil
	}

	// machine of type init bootstraps the cluster on its own,
	// unless it lost the etcd data on upgrade, in which case it joins the cluster back
	if _, err = safe.ReaderGetByID[*runtime.MetaLoaded](ctx, r, runtime.MetaLoadedID); err != nil {
		if state.IsNotFoundError(err) {
			// it is not known yet if the machine was upgraded
			return nil, nil
		}

		return nil, fmt.Errorf("error getting meta loaded: %w", err)
	}

	_, err = safe.ReaderGetByID[*runtime.MetaKey](ctx, r, runtime.MetaKeyTagToID(meta.Upgrade))

	switch {
	case err == nil:
		// upgraded
	case state.IsNotFoundError(err):
		membershipSpec.AutoBootstrap = true
	default:
		return nil, fmt.Errorf("error getting meta key: %w", err)
	}

	return membershipSpec, nil
}
