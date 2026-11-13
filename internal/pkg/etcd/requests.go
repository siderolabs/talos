// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

// leaveWithdrawTimeout bounds the time it takes to withdraw the failed leave request.
const leaveWithdrawTimeout = 5 * time.Second

// Errors returned when the etcd membership request can't be accepted.
var (
	// ErrNotReady is returned when the etcd membership controller is not ready to handle the request yet.
	ErrNotReady = errors.New("etcd is not ready yet")
	// ErrAlreadyMember is returned when the bootstrap is requested, but this machine has the etcd data already.
	ErrAlreadyMember = errors.New("etcd data directory is not empty")
	// ErrLeft is returned when the bootstrap is requested, but this machine was requested to leave the etcd cluster.
	ErrLeft = errors.New("etcd member left the cluster, the machine should be rebooted to join the cluster back")
)

// RequestBootstrap requests the etcd cluster to be bootstrapped on this machine.
//
// The request is handled asynchronously by the etcd membership controller.
//
//nolint:gocyclo
func RequestBootstrap(ctx context.Context, st state.State, spec etcd.BootstrapRequestSpec) error {
	status, err := safe.StateGetByID[*etcd.MembershipStatus](ctx, st, etcd.MembershipStatusID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return ErrNotReady
		}

		return fmt.Errorf("error getting etcd membership status: %w", err)
	}

	switch status.TypedSpec().Phase {
	case etcd.MembershipPhasePreparing:
		return ErrNotReady
	case etcd.MembershipPhaseJoining:
		// the machine is not a member of the etcd cluster, so it can bootstrap one
	case etcd.MembershipPhaseLearner, etcd.MembershipPhaseMember:
		return ErrAlreadyMember
	case etcd.MembershipPhaseLeaving, etcd.MembershipPhaseLeft:
		return ErrLeft
	}

	request := etcd.NewBootstrapRequest()
	*request.TypedSpec() = spec

	if err = st.Create(ctx, request); err != nil {
		if !state.IsConflictError(err) {
			return fmt.Errorf("error creating etcd bootstrap request: %w", err)
		}

		// there is a previous request: either it didn't go through (e.g. the snapshot was missing), or it is not relevant anymore,
		// as the etcd membership controller made its decision, but the etcd data is gone since then; replace it with the new one
		//
		// the request which is not relevant anymore is held by the controller, so wait for it to be released
		if err = st.TeardownAndDestroy(ctx, request.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error removing previous etcd bootstrap request: %w", err)
		}

		if err = st.Create(ctx, request); err != nil {
			return fmt.Errorf("error creating etcd bootstrap request: %w", err)
		}
	}

	return nil
}

// RequestLeave requests the etcd member on this machine to leave the etcd cluster, and waits for it to be done.
//
// The member is removed from the cluster, etcd is stopped, and the etcd data is wiped.
// The machine doesn't join the etcd cluster back until it is rebooted.
//
// If etcd is not configured on this machine, RequestLeave does nothing.
// If etcd is not ready (so it is not running), ErrNotReady is returned.
//
// If RequestLeave fails (e.g. the context is canceled while the etcd cluster has no quorum), the request is withdrawn,
// so that the machine doesn't leave the cluster later on behind the back of the caller. The exception is the member
// which is removed from the cluster already: there is no way back, so the etcd membership controller
// finishes leaving the cluster on its own.
//
//nolint:gocyclo
func RequestLeave(ctx context.Context, st state.State) error {
	status, err := safe.StateGetByID[*etcd.MembershipStatus](ctx, st, etcd.MembershipStatusID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// etcd is not configured
			return nil
		}

		return fmt.Errorf("error getting etcd membership status: %w", err)
	}

	if status.TypedSpec().Phase == etcd.MembershipPhasePreparing {
		// etcd is not running, and the etcd data is not available
		return ErrNotReady
	}

	// if the request is there already, it is up to its creator to withdraw it
	created := true

	if err := st.Create(ctx, etcd.NewLeaveRequest()); err != nil {
		if !state.IsConflictError(err) {
			return fmt.Errorf("error creating etcd leave request: %w", err)
		}

		created = false
	}

	var lastError string

	_, err = st.WatchFor(ctx, etcd.NewMembershipStatus().Metadata(), state.WithCondition(func(r resource.Resource) (bool, error) {
		status, ok := r.(*etcd.MembershipStatus)
		if !ok {
			return false, nil
		}

		if status.TypedSpec().Error != "" {
			lastError = status.TypedSpec().Error
		}

		return status.TypedSpec().Phase == etcd.MembershipPhaseLeft, nil
	}))
	if err != nil {
		if lastError != "" {
			err = fmt.Errorf("%s: %w", lastError, err)
		}

		if created {
			if withdrawErr := withdrawLeaveRequest(ctx, st); withdrawErr != nil {
				return fmt.Errorf("failed to leave etcd cluster: %w (the request to leave is still in effect: %w)", err, withdrawErr)
			}
		}

		return fmt.Errorf("failed to leave etcd cluster: %w", err)
	}

	return nil
}

// withdrawLeaveRequest removes the leave request, so that it is not acted upon anymore.
//
// The request which is past the point of no return (the member is removed from the cluster) is held by the finalizer
// of the etcd membership controller, so it can't be withdrawn.
func withdrawLeaveRequest(ctx context.Context, st state.State) error {
	// the context of the request is most probably done by now
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), leaveWithdrawTimeout)
	defer cancel()

	if err := st.Destroy(ctx, etcd.NewLeaveRequest().Metadata()); err != nil && !state.IsNotFoundError(err) {
		return err
	}

	return nil
}
