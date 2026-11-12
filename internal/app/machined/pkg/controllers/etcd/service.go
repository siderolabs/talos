// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"fmt"
	"slices"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/app/machined/pkg/system"
	"github.com/siderolabs/talos/internal/app/machined/pkg/system/services"
	etcdcli "github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// ServiceManager is the interface to the v1alpha1 services subsystems.
type ServiceManager interface {
	IsRunning(id string) (system.Service, bool, error)
	Load(services ...system.Service) []string
	Stop(ctx context.Context, serviceIDs ...string) (err error)
	Start(serviceIDs ...string) error
}

// ServiceController renders the final etcd service specification and controls the etcd service lifecycle.
//
// The etcd service is running if (and only if) the LaunchSpec resource exists: the service is started
// when the LaunchSpec appears, and it is stopped when the LaunchSpec is gone.
//
// If the etcd service specification changes (e.g. the etcd image or the extra arguments are changed
// in the machine configuration), the etcd service is restarted right away. The restarts are not coordinated
// across the cluster: it is up to whoever applies the machine configuration to roll the change out
// to the control plane machines one by one.
//
// The only exception is etcd running as a learner: it is never restarted, as the learner might not survive
// a restart while it is catching up with the cluster; the restart is deferred until the learner is promoted.
type ServiceController struct {
	V1Alpha1Services ServiceManager
}

// Name implements controller.Controller interface.
func (ctrl *ServiceController) Name() string {
	return "etcd.ServiceController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ServiceController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.LaunchSpecType,
			ID:        optional.Some(etcd.LaunchSpecID),
			Kind:      controller.InputWeak,
		},
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
			Namespace: etcd.NamespaceName,
			Type:      etcd.MembershipStatusType,
			ID:        optional.Some(etcd.MembershipStatusID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(etcdServiceID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ServiceController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: etcd.ServiceSpecType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *ServiceController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		current, err := safe.ReaderGetByID[*etcd.ServiceSpec](ctx, r, etcd.ServiceSpecID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting etcd service spec: %w", err)
		}

		desired, spec, err := ctrl.desiredSpec(ctx, r)
		if err != nil {
			return err
		}

		switch {
		case desired == nil && current == nil:
			// nothing to do
		case desired == nil:
			// etcd should not be running
			if err = ctrl.V1Alpha1Services.Stop(ctx, etcdServiceID); err != nil {
				return fmt.Errorf("error stopping etcd service: %w", err)
			}

			if err = r.Destroy(ctx, current.Metadata()); err != nil {
				return fmt.Errorf("error destroying etcd service spec: %w", err)
			}

			logger.Info("stopped etcd service")
		case current == nil:
			// etcd should be started
			if err = ctrl.updateSpec(ctx, r, desired); err != nil {
				return err
			}

			if err = ctrl.start(); err != nil {
				return err
			}
		case current.TypedSpec().Image == desired.Image && slices.Equal(current.TypedSpec().Args, desired.Args):
			// etcd is running with the up-to-date spec
		case current.TypedSpec().Image == desired.Image && slices.Equal(etcdcli.RestartArgs(spec, current.TypedSpec().Args), etcdcli.RestartArgs(spec, desired.Args)):
			// only the advertised URLs have changed, which is not worth restarting etcd for: the peer URLs of a running member
			// are updated by the AdvertisedPeerController, and the service spec is kept as is, as it reflects what etcd is running with
		default:
			// etcd spec has changed, etcd should be restarted
			var canRestart bool

			canRestart, err = ctrl.canRestart(ctx, r)
			if err != nil {
				return err
			}

			if !canRestart {
				logger.Info("etcd service spec has changed, but etcd is running as a learner, deferring the restart")

				break
			}

			if err = ctrl.restart(ctx, r, logger, desired); err != nil {
				return err
			}
		}

		r.ResetRestartBackoff()
	}
}

// desiredSpec builds the spec etcd should be running with (along with the etcd spec it was built from),
// it returns nil if etcd should not be running.
func (ctrl *ServiceController) desiredSpec(ctx context.Context, r controller.Runtime) (*etcd.ServiceSpecSpec, *etcd.SpecSpec, error) {
	launch, err := safe.ReaderGetByID[*etcd.LaunchSpec](ctx, r, etcd.LaunchSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil, nil
		}

		return nil, nil, fmt.Errorf("error getting etcd launch spec: %w", err)
	}

	spec, err := safe.ReaderGetByID[*etcd.Spec](ctx, r, etcd.SpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil, nil
		}

		return nil, nil, fmt.Errorf("error getting etcd spec: %w", err)
	}

	if _, err = safe.ReaderGetByID[*etcd.PKIStatus](ctx, r, etcd.PKIID); err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil, nil
		}

		return nil, nil, fmt.Errorf("error getting etcd PKI status: %w", err)
	}

	args, err := etcdcli.BuildArgs(spec.TypedSpec(), launch.TypedSpec())
	if err != nil {
		return nil, nil, fmt.Errorf("error building etcd arguments: %w", err)
	}

	return &etcd.ServiceSpecSpec{
		Image: spec.TypedSpec().Image,
		Args:  args,
	}, spec.TypedSpec(), nil
}

func (ctrl *ServiceController) updateSpec(ctx context.Context, r controller.Runtime, spec *etcd.ServiceSpecSpec) error {
	if err := safe.WriterModify(ctx, r, etcd.NewServiceSpec(), func(res *etcd.ServiceSpec) error {
		*res.TypedSpec() = *spec

		return nil
	}); err != nil {
		return fmt.Errorf("error updating etcd service spec: %w", err)
	}

	return nil
}

func (ctrl *ServiceController) start() error {
	if _, _, err := ctrl.V1Alpha1Services.IsRunning(etcdServiceID); err != nil {
		// service is not loaded yet
		ctrl.V1Alpha1Services.Load(&services.Etcd{})
	}

	if err := ctrl.V1Alpha1Services.Start(etcdServiceID); err != nil {
		return fmt.Errorf("error starting etcd service: %w", err)
	}

	return nil
}

// canRestart checks if it safe to restart etcd on this machine.
func (ctrl *ServiceController) canRestart(ctx context.Context, r controller.Runtime) (bool, error) {
	status, err := safe.ReaderGetByID[*etcd.MembershipStatus](ctx, r, etcd.MembershipStatusID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("error getting etcd membership status: %w", err)
	}

	if status.TypedSpec().Phase != etcd.MembershipPhaseLearner {
		return true, nil
	}

	// the learner which is not running (e.g. it failed to start with the current spec) has nothing to lose
	// from the restart, while the new spec might be what fixes it
	_, err = safe.ReaderGetByID[*v1alpha1.Service](ctx, r, etcdServiceID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return true, nil
		}

		return false, fmt.Errorf("error getting etcd service: %w", err)
	}

	// the service controller is woken up when the membership status or the etcd service changes
	return false, nil
}

// restart etcd with the new spec.
func (ctrl *ServiceController) restart(ctx context.Context, r controller.Runtime, logger *zap.Logger, spec *etcd.ServiceSpecSpec) error {
	logger.Info("etcd service spec has changed, restarting etcd", zap.String("image", spec.Image))

	if err := ctrl.V1Alpha1Services.Stop(ctx, etcdServiceID); err != nil {
		return fmt.Errorf("error stopping etcd service: %w", err)
	}

	if err := ctrl.updateSpec(ctx, r, spec); err != nil {
		return err
	}

	return ctrl.start()
}
