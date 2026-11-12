// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	etcdcli "github.com/siderolabs/talos/internal/pkg/etcd"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

const (
	// membershipClusterOpTimeout bounds a single operation against the etcd cluster.
	//
	// The controller handles the requests (bootstrap, leave) between the operations,
	// so the timeout also bounds the time it takes to react to a request.
	membershipClusterOpTimeout = 15 * time.Second

	membershipJoinRetryInterval    = 3 * time.Second
	membershipPromoteRetryInterval = 2 * time.Second
	membershipLeaveRetryInterval   = 5 * time.Second

	// membershipLearnerStartTimeout is the time given to etcd to start after the machine joined the cluster as a learner.
	//
	// The learner which didn't start (e.g. the etcd image can't be pulled) is removed from the cluster,
	// as it takes the place of a learner which could have joined the cluster instead.
	membershipLearnerStartTimeout = 15 * time.Minute
)

// MembershipCluster is the interface to the etcd cluster used by the MembershipController.
type MembershipCluster interface {
	// JoinAsLearner adds this machine to the etcd cluster as a learner.
	JoinAsLearner(ctx context.Context, client *etcd.ClientSpecSpec, name string, peerURLs []string) (etcdcli.JoinResult, error)
	// GetMemberState looks up the member in the etcd cluster.
	GetMemberState(ctx context.Context, client *etcd.ClientSpecSpec, memberID uint64) (etcdcli.MemberState, error)
	// RemoveMember removes the member from the etcd cluster, the member which is already gone is not an error.
	RemoveMember(ctx context.Context, client *etcd.ClientSpecSpec, memberID uint64) error
	// PromoteLearner promotes the learner to a voting member using one of the endpoints.
	PromoteLearner(ctx context.Context, client *etcd.ClientSpecSpec, endpoints []string, memberID uint64) error
}

// MembershipController manages the membership of this machine in the etcd cluster.
//
// The controller is configured with the MembershipSpec (what this etcd member is), and the ClientSpec (how to talk
// to the etcd cluster); if the MembershipSpec is gone, etcd is not configured, and the controller cleans up after itself.
//
// The controller is the only place which decides how the etcd data directory gets populated: by bootstrapping
// a new cluster (optionally recovering from a snapshot), by joining an existing cluster as a learner,
// or by using the data which is already there. The decision is published as the LaunchSpec resource,
// which is used by the ServiceController to run etcd.
//
// The controller also promotes the learner to a voting member, and handles leaving the cluster.
//
// The etcd data of a learner is disposable: if etcd was interrupted while being a learner
// (including the machine restart), the learner is removed from the cluster, the data is wiped,
// and the machine joins the cluster from scratch. The data of a voting member is never wiped,
// unless the machine is requested to leave the cluster.
type MembershipController struct {
	// Cluster is the interface to the etcd cluster, defaults to the real etcd client.
	Cluster MembershipCluster
	// SnapshotPath is the path to the etcd snapshot uploaded for the recovery.
	SnapshotPath string
	// LearnerStartTimeout is the time given to etcd to start as a learner, defaults to 15 minutes.
	LearnerStartTimeout time.Duration

	// the etcd client URLs of the voting members as seen at the moment this machine joined the cluster
	//
	// these are the URLs etcd itself advertises, so they are preferred over the discovered control plane
	// endpoints, which are node addresses that might not be running etcd at all
	votingMemberEndpoints []string
	// whether etcd was seen running as a learner
	learnerSeenRunning bool
	// the moment the controller started waiting for etcd to start as a learner
	learnerWaitingSince time.Time
	// whether the member was already removed from the cluster while leaving
	//
	// this is also recorded as the finalizer on the leave request, so it survives the controller restart
	leaveMemberRemoved bool
	// the error of the last failed attempt to recover etcd from the snapshot,
	// reported until it is decided how to launch etcd
	recoveryError string
}

// Name implements controller.Controller interface.
func (ctrl *MembershipController) Name() string {
	return "etcd.MembershipController"
}

// Inputs implements controller.Controller interface.
func (ctrl *MembershipController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.MembershipSpecType,
			ID:        optional.Some(etcd.MembershipSpecID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: etcd.ClientSpecNamespaceName,
			Type:      etcd.ClientSpecType,
			ID:        optional.Some(etcd.ClientSpecID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.BootstrapRequestType,
			ID:        optional.Some(etcd.BootstrapRequestID),
			Kind:      controller.InputStrong,
		},
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.LeaveRequestType,
			ID:        optional.Some(etcd.LeaveRequestID),
			Kind:      controller.InputStrong,
		},
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.MemberType,
			ID:        optional.Some(etcd.LocalMemberID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: etcd.NamespaceName,
			Type:      etcd.ServiceSpecType,
			ID:        optional.Some(etcd.ServiceSpecID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some(etcdServiceID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			ID:        optional.Some(ctrl.volumeMountRequestID(constants.EtcdDataVolumeID)),
			Kind:      controller.InputStrong,
		},
		{
			// the controller is woken up when the volume mount requests it tears down are ready to be destroyed
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *MembershipController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: etcd.LaunchSpecType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: etcd.MembershipStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// volumeMountRequestID returns the ID of the volume mount request (and the matching volume mount status) for the volume.
func (ctrl *MembershipController) volumeMountRequestID(volumeID string) resource.ID {
	return ctrl.Name() + "-" + volumeID
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *MembershipController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.Cluster == nil {
		ctrl.Cluster = etcdCluster{}
	}

	if ctrl.SnapshotPath == "" {
		ctrl.SnapshotPath = constants.EtcdRecoverySnapshotPath
	}

	if ctrl.LearnerStartTimeout == 0 {
		ctrl.LearnerStartTimeout = membershipLearnerStartTimeout
	}

	retryTimer := time.NewTimer(0)
	defer retryTimer.Stop()

	if !retryTimer.Stop() {
		<-retryTimer.C
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		case <-retryTimer.C:
		}

		retryAfter, err := ctrl.reconcile(ctx, r, logger)
		if err != nil {
			return err
		}

		retryTimer.Stop()

		select {
		case <-retryTimer.C:
		default:
		}

		if retryAfter > 0 {
			retryTimer.Reset(retryAfter)
		}

		r.ResetRestartBackoff()
	}
}

// membershipState is the snapshot of the inputs and the outputs of the controller for a single reconcile.
type membershipState struct {
	spec    *etcd.MembershipSpecSpec
	client  *etcd.ClientSpecSpec
	dataDir *etcdcli.DataDir

	// the current values of the controller outputs
	status etcd.MembershipStatusSpec
	launch *etcd.LaunchSpec
}

// reconcile performs a single step of the membership state machine.
//
// It returns a non-zero duration if the reconcile should be retried even if no input changes.
//
//nolint:gocyclo,cyclop
func (ctrl *MembershipController) reconcile(ctx context.Context, r controller.Runtime, logger *zap.Logger) (time.Duration, error) {
	if err := ctrl.releaseBootstrapRequest(ctx, r); err != nil {
		return 0, err
	}

	spec, err := safe.ReaderGetByID[*etcd.MembershipSpec](ctx, r, etcd.MembershipSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			// etcd is not configured (anymore), or it is not known yet how the machine should enter the cluster
			return 0, ctrl.cleanup(ctx, r)
		}

		return 0, fmt.Errorf("error getting etcd membership spec: %w", err)
	}

	// etcd is enabled, so request the ETCD volume to be mounted (with its parent volume, as the mount
	// controller doesn't mount parents on its own): the controller inspects and modifies the etcd data
	// before etcd is started, so the volume should stay mounted while the controller is active
	for _, volumeID := range []string{"/var/lib", constants.EtcdDataVolumeID} {
		if err = safe.WriterModify(
			ctx, r,
			block.NewVolumeMountRequest(block.NamespaceName, ctrl.volumeMountRequestID(volumeID)),
			func(v *block.VolumeMountRequest) error {
				v.TypedSpec().Requester = ctrl.Name()
				v.TypedSpec().VolumeID = volumeID

				return nil
			},
		); err != nil {
			return 0, fmt.Errorf("error creating volume mount request for %q: %w", volumeID, err)
		}
	}

	mountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, ctrl.volumeMountRequestID(constants.EtcdDataVolumeID))
	if err != nil {
		if state.IsNotFoundError(err) {
			// ETCD volume is not mounted yet
			return 0, ctrl.notReady(ctx, r)
		}

		return 0, fmt.Errorf("error getting volume mount status for the ETCD volume: %w", err)
	}

	switch mountStatus.Metadata().Phase() {
	case resource.PhaseTearingDown:
		// the ETCD volume is being unmounted: etcd should not be running, and the decision on how to launch etcd
		// should be taken again once the volume is back, as the data might be gone
		if err = ctrl.notReady(ctx, r); err != nil {
			return 0, err
		}

		if mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
			if err = r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
				return 0, fmt.Errorf("error removing finalizer from volume mount status for the ETCD volume: %w", err)
			}
		}

		return 0, nil
	case resource.PhaseRunning:
		if !mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
			if err = r.AddFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
				return 0, fmt.Errorf("error adding finalizer to volume mount status for the ETCD volume: %w", err)
			}

			// the finalizer update triggers another reconcile
			return 0, nil
		}
	}

	// the etcd client is required to talk to the etcd cluster
	clientSpec, err := safe.ReaderGetByID[*etcd.ClientSpec](ctx, r, etcd.ClientSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return 0, ctrl.notReady(ctx, r)
		}

		return 0, fmt.Errorf("error getting etcd client spec: %w", err)
	}

	st := membershipState{
		spec:    spec.TypedSpec(),
		client:  clientSpec.TypedSpec(),
		dataDir: etcdcli.NewDataDir(mountStatus.TypedSpec().Target),
	}

	status, err := safe.ReaderGetByID[*etcd.MembershipStatus](ctx, r, etcd.MembershipStatusID)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, fmt.Errorf("error getting etcd membership status: %w", err)
	}

	if status != nil {
		st.status = *status.TypedSpec()
	}

	st.launch, err = safe.ReaderGetByID[*etcd.LaunchSpec](ctx, r, etcd.LaunchSpecID)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, fmt.Errorf("error getting etcd launch spec: %w", err)
	}

	if st.launch != nil {
		// it is decided how to launch etcd, so the bootstrap request (if any) is not going to be acted upon
		if err = ctrl.consumeBootstrapRequest(ctx, r); err != nil {
			return 0, err
		}
	}

	leaveRequest, err := safe.ReaderGetByID[*etcd.LeaveRequest](ctx, r, etcd.LeaveRequestID)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, fmt.Errorf("error getting etcd leave request: %w", err)
	}

	switch {
	case leaveRequest != nil:
		return ctrl.reconcileLeave(ctx, r, logger, st, leaveRequest)
	case ctrl.leaveMemberRemoved && st.status.Phase == etcd.MembershipPhaseLeaving:
		// the leave request was withdrawn, but the member is removed from the cluster already,
		// so there is no way back: finish leaving the cluster, and join it back from scratch
		return ctrl.reconcileLeave(ctx, r, logger, st, nil)
	}

	ctrl.leaveMemberRemoved = false

	if st.launch == nil {
		return ctrl.reconcileLaunch(ctx, r, logger, st)
	}

	// etcd is launched: the learner marker (and not the last reported phase) tells a learner from a voting member,
	// as the marker is kept in sync with the membership in the etcd cluster, while the phase might be stale,
	// e.g. if the controller failed halfway, or if the leave request was withdrawn
	learnerMemberID, isLearner, err := st.dataDir.LearnerMarker()
	if err != nil {
		return 0, fmt.Errorf("error reading etcd learner marker: %w", err)
	}

	if isLearner {
		return ctrl.reconcileLearner(ctx, r, logger, st, learnerMemberID)
	}

	// voting member: keep the member ID in sync
	return 0, ctrl.updateMemberStatus(ctx, r, st.status.MemberID)
}

// releaseBootstrapRequest lets go of the bootstrap request which is being replaced with a new one.
func (ctrl *MembershipController) releaseBootstrapRequest(ctx context.Context, r controller.Runtime) error {
	request, err := safe.ReaderGetByID[*etcd.BootstrapRequest](ctx, r, etcd.BootstrapRequestID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error getting etcd bootstrap request: %w", err)
	}

	if request.Metadata().Phase() != resource.PhaseTearingDown || !request.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	if err = r.RemoveFinalizer(ctx, request.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("error removing finalizer from etcd bootstrap request: %w", err)
	}

	return nil
}

// pendingBootstrapRequest returns the bootstrap request which is still to be acted upon (if any).
//
// The bootstrap request is a one-shot: it should not bootstrap another cluster if the etcd data is gone
// for whatever reason later on. The request which can't be acted upon anymore is marked with the finalizer
// of the controller, so the mark is not lost if the controller restarts.
func (ctrl *MembershipController) pendingBootstrapRequest(ctx context.Context, r controller.Runtime) (*etcd.BootstrapRequest, error) {
	request, err := safe.ReaderGetByID[*etcd.BootstrapRequest](ctx, r, etcd.BootstrapRequestID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("error getting etcd bootstrap request: %w", err)
	}

	if request.Metadata().Phase() != resource.PhaseRunning || request.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil, nil
	}

	return request, nil
}

// consumeBootstrapRequest marks the pending bootstrap request (if any) as the one which can't be acted upon anymore.
func (ctrl *MembershipController) consumeBootstrapRequest(ctx context.Context, r controller.Runtime) error {
	request, err := ctrl.pendingBootstrapRequest(ctx, r)
	if err != nil || request == nil {
		return err
	}

	if err = r.AddFinalizer(ctx, request.Metadata(), ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error adding finalizer to etcd bootstrap request: %w", err)
	}

	return nil
}

// updateMemberStatus reports the machine as a (voting) member of the etcd cluster.
//
// The member ID is reported by etcd itself once it is healthy; until then, the last known ID (if any) is kept.
func (ctrl *MembershipController) updateMemberStatus(ctx context.Context, r controller.Runtime, lastKnownMemberID string) error {
	memberID := lastKnownMemberID

	member, err := safe.ReaderGetByID[*etcd.Member](ctx, r, etcd.LocalMemberID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting etcd member: %w", err)
	}

	if member != nil {
		memberID = member.TypedSpec().MemberID
	}

	return ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
		Phase:    etcd.MembershipPhaseMember,
		MemberID: memberID,
	})
}

// cleanup removes all outputs of the controller, as etcd is not configured.
//
//nolint:gocyclo
func (ctrl *MembershipController) cleanup(ctx context.Context, r controller.Runtime) error {
	ctrl.resetState()

	for _, md := range []resource.Pointer{
		etcd.NewLaunchSpec().Metadata(),
		etcd.NewMembershipStatus().Metadata(),
	} {
		if err := r.Destroy(ctx, md); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error destroying %s: %w", md, err)
		}
	}

	mountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, ctrl.volumeMountRequestID(constants.EtcdDataVolumeID))
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error getting volume mount status for the ETCD volume: %w", err)
	}

	if mountStatus != nil && mountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		if err = r.RemoveFinalizer(ctx, mountStatus.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error removing finalizer from volume mount status for the ETCD volume: %w", err)
		}
	}

	for _, volumeID := range []string{constants.EtcdDataVolumeID, "/var/lib"} {
		requestMD := block.NewVolumeMountRequest(block.NamespaceName, ctrl.volumeMountRequestID(volumeID)).Metadata()

		okToDestroy, err := r.Teardown(ctx, requestMD)
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("error tearing down volume mount request for %q: %w", volumeID, err)
		}

		if !okToDestroy {
			// the request is destroyed once the volume is unmounted (and the request is released),
			// the controller is woken up when the request is ready to be destroyed
			continue
		}

		if err = r.Destroy(ctx, requestMD); err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error destroying volume mount request for %q: %w", volumeID, err)
		}
	}

	return nil
}

// notReady handles the case when the prerequisites are not ready: etcd should not be running.
func (ctrl *MembershipController) notReady(ctx context.Context, r controller.Runtime) error {
	ctrl.resetState()

	if err := r.Destroy(ctx, etcd.NewLaunchSpec().Metadata()); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error destroying etcd launch spec: %w", err)
	}

	return ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
		Phase: etcd.MembershipPhasePreparing,
	})
}

func (ctrl *MembershipController) resetState() {
	ctrl.votingMemberEndpoints = nil
	ctrl.learnerSeenRunning = false
	ctrl.learnerWaitingSince = time.Time{}
	ctrl.leaveMemberRemoved = false
}

func (ctrl *MembershipController) updateStatus(ctx context.Context, r controller.Runtime, spec etcd.MembershipStatusSpec) error {
	if err := safe.WriterModify(ctx, r, etcd.NewMembershipStatus(), func(status *etcd.MembershipStatus) error {
		*status.TypedSpec() = spec

		return nil
	}); err != nil {
		return fmt.Errorf("error updating etcd membership status: %w", err)
	}

	return nil
}

func (ctrl *MembershipController) createLaunchSpec(ctx context.Context, r controller.Runtime, spec etcd.LaunchSpecSpec) error {
	if err := safe.WriterModify(ctx, r, etcd.NewLaunchSpec(), func(launch *etcd.LaunchSpec) error {
		*launch.TypedSpec() = spec

		return nil
	}); err != nil {
		return fmt.Errorf("error updating etcd launch spec: %w", err)
	}

	ctrl.recoveryError = ""

	// whatever the way to launch etcd is, the bootstrap request should not be acted upon after this point
	return ctrl.consumeBootstrapRequest(ctx, r)
}

// serviceStopped checks that the etcd service is stopped by the ServiceController.
func (ctrl *MembershipController) serviceStopped(ctx context.Context, r controller.Runtime) (bool, error) {
	_, err := safe.ReaderGetByID[*etcd.ServiceSpec](ctx, r, etcd.ServiceSpecID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return true, nil
		}

		return false, fmt.Errorf("error getting etcd service spec: %w", err)
	}

	return false, nil
}

// reconcileLaunch decides how etcd should be launched, populating the etcd data directory if required.
//
//nolint:gocyclo,cyclop
func (ctrl *MembershipController) reconcileLaunch(ctx context.Context, r controller.Runtime, logger *zap.Logger, st membershipState) (time.Duration, error) {
	// the etcd data should not be touched while etcd is still running
	stopped, err := ctrl.serviceStopped(ctx, r)
	if err != nil {
		return 0, err
	}

	if !stopped {
		// wait for the service to be stopped, the controller will be woken up
		return 0, nil
	}

	markerMemberID, hasMarker, err := st.dataDir.LearnerMarker()
	if err != nil {
		return 0, fmt.Errorf("error reading etcd learner marker: %w", err)
	}

	if hasMarker {
		// etcd was interrupted while it was a learner (or right after the promotion)
		if err = ctrl.resolveLearnerMarker(ctx, logger, st, markerMemberID); err != nil {
			logger.Warn("failed to clean up after the interrupted etcd learner", zap.Error(err))

			return membershipJoinRetryInterval, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
				Phase:    etcd.MembershipPhaseJoining,
				MemberID: etcd.FormatMemberID(markerMemberID),
				Error:    err.Error(),
			})
		}
	}

	hasData, err := st.dataDir.HasMemberData()
	if err != nil {
		return 0, fmt.Errorf("error checking etcd data directory: %w", err)
	}

	if hasData {
		// etcd data is already there, etcd picks up the cluster configuration from it
		if err = ctrl.createLaunchSpec(ctx, r, etcd.LaunchSpecSpec{
			InitialClusterState: etcdcli.InitialClusterStateExisting,
		}); err != nil {
			return 0, err
		}

		return 0, ctrl.updateMemberStatus(ctx, r, "")
	}

	// etcd data is empty
	bootstrapRequest, err := ctrl.pendingBootstrapRequest(ctx, r)
	if err != nil {
		return 0, err
	}

	peerURLs := etcdcli.FormatURLs(st.spec.AdvertisedAddresses, constants.EtcdPeerPort)

	switch {
	case bootstrapRequest != nil && bootstrapRequest.TypedSpec().RecoverFromSnapshot:
		// bootstrap the cluster from the snapshot
		if err = etcdcli.RestoreSnapshot(logger, etcdcli.RestoreSnapshotOptions{
			SnapshotPath:  ctrl.SnapshotPath,
			DataDir:       st.dataDir.Path(),
			Name:          st.spec.Name,
			PeerURLs:      peerURLs,
			SkipHashCheck: bootstrapRequest.TypedSpec().RecoverSkipHashCheck,
		}); err != nil {
			logger.Error("failed to recover etcd from the snapshot, the bootstrap should be requested once again", zap.Error(err))

			// make sure a partially restored data is not picked up as the etcd data
			if wipeErr := st.dataDir.Wipe(); wipeErr != nil {
				return 0, fmt.Errorf("error wiping etcd data directory: %w", wipeErr)
			}

			// the recovery is not retried, as it is not going to succeed with the same snapshot:
			// the request is done with, and the machine goes back to waiting to join the cluster (or to be bootstrapped)
			if err = ctrl.consumeBootstrapRequest(ctx, r); err != nil {
				return 0, err
			}

			ctrl.recoveryError = fmt.Sprintf("failed to recover etcd from the snapshot: %s", err)

			return membershipJoinRetryInterval, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
				Phase: etcd.MembershipPhaseJoining,
				Error: ctrl.recoveryError,
			})
		}

		logger.Info("bootstrapping etcd cluster from the snapshot")

		return 0, ctrl.launchAsMember(ctx, r, etcd.LaunchSpecSpec{
			InitialClusterState: etcdcli.InitialClusterStateExisting,
		})
	case st.spec.ManualInitialCluster:
		// the initial cluster is managed outside of Talos, so there is nothing to do with the cluster membership
		initialClusterState := etcdcli.InitialClusterStateExisting

		if bootstrapRequest != nil || st.spec.InitNode {
			initialClusterState = etcdcli.InitialClusterStateNew
		}

		return 0, ctrl.launchAsMember(ctx, r, etcd.LaunchSpecSpec{
			InitialClusterState: initialClusterState,
		})
	case bootstrapRequest != nil:
		logger.Info("bootstrapping etcd cluster")

		return 0, ctrl.launchAsMember(ctx, r, etcd.LaunchSpecSpec{
			InitialClusterState:      etcdcli.InitialClusterStateNew,
			InitialCluster:           etcdcli.FormatInitialCluster(st.spec.Name, peerURLs),
			InitialAdvertisePeerURLs: peerURLs,
		})
	case st.spec.AutoBootstrap:
		// machine of type init bootstraps the cluster on its own,
		// unless it lost the etcd data on upgrade, in which case it joins the cluster back
		logger.Info("bootstrapping etcd cluster")

		return 0, ctrl.launchAsMember(ctx, r, etcd.LaunchSpecSpec{
			InitialClusterState:      etcdcli.InitialClusterStateNew,
			InitialCluster:           etcdcli.FormatInitialCluster(st.spec.Name, peerURLs),
			InitialAdvertisePeerURLs: peerURLs,
		})
	}

	return ctrl.join(ctx, r, logger, st, peerURLs)
}

func (ctrl *MembershipController) launchAsMember(ctx context.Context, r controller.Runtime, launchSpec etcd.LaunchSpecSpec) error {
	if err := ctrl.createLaunchSpec(ctx, r, launchSpec); err != nil {
		return err
	}

	return ctrl.updateMemberStatus(ctx, r, "")
}

// resolveLearnerMarker cleans up after etcd which was interrupted while being a learner.
//
// The learner is not a part of the quorum, so its data is disposable: the learner is removed from the cluster,
// and the data is wiped. If the learner turns out to be promoted (the promotion and the removal
// of the marker are not atomic), the data is kept.
func (ctrl *MembershipController) resolveLearnerMarker(ctx context.Context, logger *zap.Logger, st membershipState, memberID uint64) error {
	ctx, cancel := context.WithTimeout(ctx, membershipClusterOpTimeout)
	defer cancel()

	memberState := etcdcli.MemberStateAbsent

	if memberID != 0 {
		var err error

		memberState, err = ctrl.Cluster.GetMemberState(ctx, st.client, memberID)
		if err != nil {
			return fmt.Errorf("error getting state of the etcd learner %s: %w", etcd.FormatMemberID(memberID), err)
		}
	}

	switch memberState {
	case etcdcli.MemberStateVoting:
		logger.Info("etcd member is promoted already, keeping the etcd data", zap.String("member_id", etcd.FormatMemberID(memberID)))

		if err := st.dataDir.RemoveLearnerMarker(); err != nil {
			return fmt.Errorf("error removing etcd learner marker: %w", err)
		}

		return nil
	case etcdcli.MemberStateLearner:
		if err := ctrl.Cluster.RemoveMember(ctx, st.client, memberID); err != nil {
			return fmt.Errorf("error removing etcd learner %s: %w", etcd.FormatMemberID(memberID), err)
		}
	case etcdcli.MemberStateAbsent:
	}

	logger.Info("etcd was interrupted while being a learner, wiping the etcd data", zap.String("member_id", etcd.FormatMemberID(memberID)))

	if err := st.dataDir.Wipe(); err != nil {
		return fmt.Errorf("error wiping etcd data directory: %w", err)
	}

	return nil
}

// join attempts to join the existing etcd cluster as a learner.
func (ctrl *MembershipController) join(ctx context.Context, r controller.Runtime, logger *zap.Logger, st membershipState, peerURLs []string) (time.Duration, error) {
	if st.status.Phase != etcd.MembershipPhaseJoining {
		// report the phase before the (potentially slow) attempt to join the cluster,
		// so that the bootstrap request is accepted as soon as possible
		if err := ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
			Phase: etcd.MembershipPhaseJoining,
		}); err != nil {
			return 0, err
		}
	}

	joinCtx, cancel := context.WithTimeout(ctx, membershipClusterOpTimeout)
	defer cancel()

	result, err := ctrl.Cluster.JoinAsLearner(joinCtx, st.client, st.spec.Name, peerURLs)
	if err != nil {
		if ctx.Err() != nil {
			return 0, nil
		}

		logger.Debug("failed to join the etcd cluster", zap.Error(err))

		joinError := err.Error()

		if ctrl.recoveryError != "" {
			// keep the recovery failure visible, as it is what should be acted upon
			joinError = ctrl.recoveryError + "; " + joinError
		}

		return membershipJoinRetryInterval, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
			Phase: etcd.MembershipPhaseJoining,
			Error: joinError,
		})
	}

	// mark the data as disposable before etcd is started
	if err = st.dataDir.WriteLearnerMarker(result.LearnerMemberID); err != nil {
		return 0, fmt.Errorf("error writing etcd learner marker: %w", err)
	}

	logger.Info("joined the etcd cluster as a learner", zap.String("member_id", etcd.FormatMemberID(result.LearnerMemberID)))

	ctrl.votingMemberEndpoints = result.VotingMemberEndpoints
	ctrl.learnerSeenRunning = false
	ctrl.learnerWaitingSince = time.Time{}

	if err = ctrl.createLaunchSpec(ctx, r, etcd.LaunchSpecSpec{
		InitialClusterState:      etcdcli.InitialClusterStateExisting,
		InitialCluster:           result.InitialCluster,
		InitialAdvertisePeerURLs: peerURLs,
	}); err != nil {
		return 0, err
	}

	return 0, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
		Phase:    etcd.MembershipPhaseLearner,
		MemberID: etcd.FormatMemberID(result.LearnerMemberID),
	})
}

// reconcileLearner watches over the etcd learner, and promotes it to a voting member.
//
//nolint:gocyclo
func (ctrl *MembershipController) reconcileLearner(ctx context.Context, r controller.Runtime, logger *zap.Logger, st membershipState, memberID uint64) (time.Duration, error) {
	formattedMemberID := etcd.FormatMemberID(memberID)

	_, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, etcdServiceID)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, fmt.Errorf("error getting etcd service: %w", err)
	}

	running := err == nil

	if !running || memberID == 0 {
		reason := "etcd was interrupted while being a learner"

		if !ctrl.learnerSeenRunning && memberID != 0 {
			// etcd is not started yet, give it some time
			if ctrl.learnerWaitingSince.IsZero() {
				ctrl.learnerWaitingSince = time.Now()
			}

			if remaining := ctrl.LearnerStartTimeout - time.Since(ctrl.learnerWaitingSince); remaining > 0 {
				if st.status.Phase == etcd.MembershipPhaseLearner && st.status.MemberID == formattedMemberID {
					return remaining, nil
				}

				return remaining, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
					Phase:    etcd.MembershipPhaseLearner,
					MemberID: formattedMemberID,
				})
			}

			reason = "etcd failed to start as a learner"
		}

		// either etcd was running, and now it is not (a learner might not survive a restart while it is catching up
		// with the cluster), or it didn't start in time, or the member ID of the learner is lost (so it can't be promoted):
		// stop it, and start from scratch
		logger.Warn(reason + ", re-joining the etcd cluster")

		ctrl.learnerSeenRunning = false
		ctrl.learnerWaitingSince = time.Time{}
		ctrl.votingMemberEndpoints = nil

		if err = r.Destroy(ctx, st.launch.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return 0, fmt.Errorf("error destroying etcd launch spec: %w", err)
		}

		// the learner marker is still there, so the cleanup is done once the service is stopped
		return membershipJoinRetryInterval, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
			Phase:    etcd.MembershipPhaseJoining,
			MemberID: formattedMemberID,
			Error:    reason,
		})
	}

	ctrl.learnerSeenRunning = true
	ctrl.learnerWaitingSince = time.Time{}

	endpoints := etcdcli.PromotionEndpoints(
		etcdcli.FormatURLs(st.spec.AdvertisedAddresses, constants.EtcdClientPort),
		ctrl.votingMemberEndpoints,
		// the discovered endpoints are optional, as long as the endpoints of the voting members are known
		st.client.ClusterEndpoints,
	)

	promoteCtx, cancel := context.WithTimeout(ctx, 2*membershipClusterOpTimeout)
	defer cancel()

	if err = ctrl.Cluster.PromoteLearner(promoteCtx, st.client, endpoints, memberID); err != nil {
		if ctx.Err() != nil {
			return 0, nil
		}

		logger.Debug("failed to promote the etcd learner", zap.Error(err))

		// the promotion fails while the learner is catching up with the cluster
		return membershipPromoteRetryInterval, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
			Phase:    etcd.MembershipPhaseLearner,
			MemberID: formattedMemberID,
			Error:    err.Error(),
		})
	}

	// the member is a voting member now, so its data is not disposable anymore
	//
	// the promotion and the removal of the marker are not atomic: if the machine goes down in between, the marker
	// is resolved against the etcd cluster on the next boot, which requires the cluster to have the quorum without
	// this member; if it doesn't, etcd is not started, and the way out is to remove this member from the cluster
	// (on another machine), and let it join back
	if err = st.dataDir.RemoveLearnerMarker(); err != nil {
		return 0, fmt.Errorf("error removing etcd learner marker: %w", err)
	}

	logger.Info("successfully promoted etcd member", zap.String("member_id", formattedMemberID))

	ctrl.learnerSeenRunning = false
	ctrl.votingMemberEndpoints = nil

	return 0, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
		Phase:    etcd.MembershipPhaseMember,
		MemberID: formattedMemberID,
	})
}

// reconcileLeave handles the request to leave the etcd cluster.
//
// The leave request can be withdrawn (destroyed) until the member is removed from the etcd cluster: this is the point
// of no return, which is recorded as the finalizer of the controller on the request. After that the machine
// leaves the cluster even if the request is gone (leaveRequest is nil).
//
//nolint:gocyclo,cyclop
func (ctrl *MembershipController) reconcileLeave(ctx context.Context, r controller.Runtime, logger *zap.Logger, st membershipState, leaveRequest *etcd.LeaveRequest) (time.Duration, error) {
	if st.status.Phase == etcd.MembershipPhaseLeft {
		// the machine stays out of the cluster while the request is there
		return 0, nil
	}

	committed := leaveRequest != nil && leaveRequest.Metadata().Finalizers().Has(ctrl.Name())

	if committed {
		ctrl.leaveMemberRemoved = true
	}

	if !ctrl.leaveMemberRemoved {
		memberID, known, err := ctrl.leavingMemberID(ctx, r, st)
		if err != nil {
			return 0, err
		}

		if !known {
			// there is etcd data, but etcd didn't report the member ID yet
			return membershipLeaveRetryInterval, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
				Phase: etcd.MembershipPhaseLeaving,
				Error: "local etcd member ID is not known yet",
			})
		}

		if memberID != 0 {
			removeCtx, cancel := context.WithTimeout(ctx, membershipClusterOpTimeout)
			defer cancel()

			if err = ctrl.Cluster.RemoveMember(removeCtx, st.client, memberID); err != nil {
				if ctx.Err() != nil {
					return 0, nil
				}

				logger.Warn("failed to remove the member from the etcd cluster", zap.String("member_id", etcd.FormatMemberID(memberID)), zap.Error(err))

				return membershipLeaveRetryInterval, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
					Phase:    etcd.MembershipPhaseLeaving,
					MemberID: etcd.FormatMemberID(memberID),
					Error:    err.Error(),
				})
			}

			logger.Info("removed the member from the etcd cluster", zap.String("member_id", etcd.FormatMemberID(memberID)))
		}

		ctrl.leaveMemberRemoved = true
	}

	if leaveRequest != nil && !committed {
		// if the request was withdrawn while the member was being removed, the machine still leaves the cluster
		if err := r.AddFinalizer(ctx, leaveRequest.Metadata(), ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
			return 0, fmt.Errorf("error adding finalizer to etcd leave request: %w", err)
		}
	}

	// once the member is removed, etcd should be stopped
	if st.launch != nil {
		if err := r.Destroy(ctx, st.launch.Metadata()); err != nil && !state.IsNotFoundError(err) {
			return 0, fmt.Errorf("error destroying etcd launch spec: %w", err)
		}
	}

	stopped, err := ctrl.serviceStopped(ctx, r)
	if err != nil {
		return 0, err
	}

	if !stopped {
		// wait for the service to be stopped, the controller will be woken up
		return 0, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
			Phase:    etcd.MembershipPhaseLeaving,
			MemberID: st.status.MemberID,
		})
	}

	// the member is removed, so the data is no longer valid
	if err = st.dataDir.Wipe(); err != nil {
		return 0, fmt.Errorf("error wiping etcd data directory: %w", err)
	}

	logger.Info("left the etcd cluster")

	ctrl.resetState()

	var retryAfter time.Duration

	if leaveRequest == nil {
		// nothing keeps the machine out of the cluster, so join it back
		retryAfter = membershipJoinRetryInterval
	}

	return retryAfter, ctrl.updateStatus(ctx, r, etcd.MembershipStatusSpec{
		Phase: etcd.MembershipPhaseLeft,
	})
}

// leavingMemberID figures out the ID of the etcd member to remove from the cluster.
//
// It returns zero ID if the machine is not a member of the cluster, and false if the member ID is not known (yet).
//
//nolint:gocyclo
func (ctrl *MembershipController) leavingMemberID(ctx context.Context, r controller.Runtime, st membershipState) (uint64, bool, error) {
	member, err := safe.ReaderGetByID[*etcd.Member](ctx, r, etcd.LocalMemberID)
	if err != nil && !state.IsNotFoundError(err) {
		return 0, false, fmt.Errorf("error getting etcd member: %w", err)
	}

	for _, id := range []string{
		func() string {
			if member != nil {
				return member.TypedSpec().MemberID
			}

			return ""
		}(),
		st.status.MemberID,
	} {
		if id == "" {
			continue
		}

		memberID, err := etcd.ParseMemberID(id)
		if err != nil {
			return 0, false, err
		}

		return memberID, true, nil
	}

	markerMemberID, hasMarker, err := st.dataDir.LearnerMarker()
	if err != nil {
		return 0, false, fmt.Errorf("error reading etcd learner marker: %w", err)
	}

	if hasMarker && markerMemberID != 0 {
		return markerMemberID, true, nil
	}

	hasData, err := st.dataDir.HasMemberData()
	if err != nil {
		return 0, false, fmt.Errorf("error checking etcd data directory: %w", err)
	}

	// no data: the machine is not a member, so there is nothing to remove from the cluster
	return 0, !hasData, nil
}

// etcdCluster implements MembershipCluster with the real etcd client.
type etcdCluster struct{}

func (etcdCluster) withClient(ctx context.Context, clientSpec *etcd.ClientSpecSpec, f func(*etcdcli.Client) error) error {
	client, err := etcdcli.NewClusterClientFromSpec(ctx, clientSpec)
	if err != nil {
		return err
	}

	defer client.Close() //nolint:errcheck

	return f(client)
}

func (c etcdCluster) JoinAsLearner(ctx context.Context, clientSpec *etcd.ClientSpecSpec, name string, peerURLs []string) (etcdcli.JoinResult, error) {
	var result etcdcli.JoinResult

	err := c.withClient(ctx, clientSpec, func(client *etcdcli.Client) error {
		var err error

		result, err = client.JoinAsLearner(ctx, name, peerURLs)

		return err
	})

	return result, err
}

func (c etcdCluster) GetMemberState(ctx context.Context, clientSpec *etcd.ClientSpecSpec, memberID uint64) (etcdcli.MemberState, error) {
	var result etcdcli.MemberState

	err := c.withClient(ctx, clientSpec, func(client *etcdcli.Client) error {
		var err error

		result, err = client.GetMemberState(ctx, memberID)

		return err
	})

	return result, err
}

func (c etcdCluster) RemoveMember(ctx context.Context, clientSpec *etcd.ClientSpecSpec, memberID uint64) error {
	return c.withClient(ctx, clientSpec, func(client *etcdcli.Client) error {
		return client.RemoveMemberIfExists(ctx, memberID)
	})
}

func (etcdCluster) PromoteLearner(ctx context.Context, clientSpec *etcd.ClientSpecSpec, endpoints []string, memberID uint64) error {
	if len(endpoints) == 0 {
		return errors.New("no etcd endpoints to promote the learner")
	}

	return etcdcli.PromoteLearner(ctx, clientSpec, endpoints, memberID)
}
