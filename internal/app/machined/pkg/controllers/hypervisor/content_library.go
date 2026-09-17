// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hypervisor provides controllers for the Talos hypervisor.
package hypervisor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/contentlibrary/staging"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// ContentLibraryController mounts the volume backing each configured content library.
//
// A library's contents sit at the root of the mount target, with no directory of the library's own,
// so a volume backs exactly one library. The mount is held for as long as the library is configured,
// and given back when the library goes away or is pointed at a different volume.
type ContentLibraryController struct{}

// Name implements controller.Controller interface.
func (ctrl *ContentLibraryController) Name() string {
	return "hypervisor.ContentLibraryController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ContentLibraryController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        optional.Some(config.ActiveID),
			Kind:      controller.InputWeak,
		},
		{
			// Strong, as holding a mount means putting a finalizer on its status.
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountStatusType,
			Kind:      controller.InputStrong,
		},
		{
			// Destroy-ready, so a request this controller tore down wakes it once nothing holds it.
			Namespace: block.NamespaceName,
			Type:      block.VolumeMountRequestType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ContentLibraryController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.ContentLibraryStatusType,
			Kind: controller.OutputExclusive,
		},
		{
			Type: block.VolumeMountRequestType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *ContentLibraryController) Run(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		}

		if err := ctrl.reconcile(ctx, runtime, logger); err != nil {
			logger.Error("failed to reconcile content libraries", zap.Error(err))

			return err
		}

		runtime.ResetRestartBackoff()
	}
}

func (ctrl *ContentLibraryController) reconcile(ctx context.Context, runtime controller.Runtime, logger *zap.Logger) error {
	machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, runtime, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get machine config: %w", err)
	}

	runtime.StartTrackingOutputs()

	wantedMountRequests := map[string]struct{}{}

	if machineConfig != nil && machineConfig.Config() != nil {
		for _, contentLibraryConfig := range machineConfig.Config().ContentLibraryConfigs() {
			requestID, err := ctrl.reconcileLibrary(ctx, runtime, logger, machineConfig.Config(), contentLibraryConfig)
			if err != nil {
				return err
			}

			if requestID != "" {
				wantedMountRequests[requestID] = struct{}{}
			}
		}
	}

	// The mounts of libraries which are no longer configured are given back here, before the statuses
	// are cleaned up.
	if err := ctrl.releaseUnwanted(ctx, runtime, logger, wantedMountRequests); err != nil {
		return err
	}

	// Mount requests are deliberately not swept as tracked outputs: CleanupOutputs destroys without
	// tearing down first, and leaves no place to drop the finalizer on the mount status, which would
	// deadlock the unmount.
	if err := safe.CleanupOutputs[*hypervisor.ContentLibraryStatus](ctx, runtime); err != nil {
		return fmt.Errorf("failed to clean up outputs: %w", err)
	}

	return nil
}

// reconcileLibrary brings one content library up and reports what came of it on its status.
//
// Returns the ID of the mount request the library wants, empty when it wants none.
func (ctrl *ContentLibraryController) reconcileLibrary(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	cfg configcfg.Config,
	contentLibraryConfig configcfg.ContentLibraryConfig,
) (string, error) {
	libraryID := contentLibraryConfig.Name()
	volumeName := contentLibraryConfig.BackingVolumeName()

	backing, resolved := resolveBackingVolume(cfg, volumeName)

	var (
		requestID string
		path      string
		reason    string
	)

	switch {
	case !resolved:
		// Unresolved is as far as this gets: without a volume there is nothing to mount, and nothing
		// may be requested from the block subsystem for a volume nothing declares. Configuration
		// validation rejects a backing volume nothing declares, so this is what a library says while
		// its status is written before it.
		reason = fmt.Sprintf("no user, existing or external volume named %q is configured", volumeName)
	case backing.readOnly:
		// Nothing is asked for: a mount is read-only only while every requester asks for it, so the
		// read-write mount a library needs would remount the volume against what the user declared.
		reason = fmt.Sprintf("backing volume %q is configured read-only", volumeName)
	default:
		requestID = ctrl.mountRequestID(libraryID, backing.id)

		var err error

		if path, reason, err = ctrl.mount(ctx, runtime, logger, libraryID, backing.id, requestID); err != nil {
			return "", err
		}
	}

	// Sweeping is only safe on the transition to ready, so the transition is noticed here, where the
	// previous status is still readable.
	var becameReady bool

	if err := safe.WriterModify(ctx, runtime,
		hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryID),
		func(res *hypervisor.ContentLibraryStatus) error {
			becameReady = reason == "" && !res.TypedSpec().Ready

			res.TypedSpec().VolumeID = backing.id
			res.TypedSpec().Path = path
			res.TypedSpec().Ready = reason == ""
			res.TypedSpec().Error = reason

			return nil
		},
	); err != nil {
		return "", fmt.Errorf("failed to write content library status %q: %w", libraryID, err)
	}

	if becameReady {
		ctrl.sweepStagedUploads(logger, libraryID, path)
	}

	return requestID, nil
}

// mountRequestID builds the ID of the mount request for one library and its backing volume.
//
// Slash-separated because library names and volume IDs both contain hyphens, which would make a
// hyphen-joined ID ambiguous. The volume is part of the ID so that pointing a library at another
// volume asks for a new mount rather than mutating a request the block subsystem has already acted
// on; the old request is then released as unwanted.
func (ctrl *ContentLibraryController) mountRequestID(libraryID, volumeID string) string {
	return ctrl.Name() + "/" + libraryID + "/" + volumeID
}

// mount asks for the backing volume to be mounted and holds the mount once it appears.
//
// Returns the path the library's contents sit at, and the reason it is not ready yet, empty when it
// is.
func (ctrl *ContentLibraryController) mount(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	libraryID, volumeID, requestID string,
) (string, string, error) {
	if err := ctrl.requestMount(ctx, runtime, volumeID, requestID); err != nil {
		return "", "", err
	}

	volumeMountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, runtime, requestID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", "waiting for the backing volume to be mounted", nil
		}

		return "", "", fmt.Errorf("failed to get mount status %q: %w", requestID, err)
	}

	return ctrl.setMountFinalizer(ctx, runtime, logger, libraryID, volumeID, volumeMountStatus)
}

// requestMount asks the block subsystem to mount the backing volume.
//
// Written whether or not the volume is ready: the request is what makes it mounted once it is.
func (ctrl *ContentLibraryController) requestMount(ctx context.Context, runtime controller.Runtime, volumeID, requestID string) error {
	if err := safe.WriterModify(ctx, runtime,
		block.NewVolumeMountRequest(block.NamespaceName, requestID),
		func(res *block.VolumeMountRequest) error {
			res.TypedSpec().Requester = ctrl.Name()
			res.TypedSpec().VolumeID = volumeID
			// Uploads write into the library, so read-write. Asked for unconditionally because a
			// volume the user declared read-only is refused before a mount is asked for: a mount is
			// read-only only while every requester asks for it, so a read-write request would
			// remount such a volume.
			res.TypedSpec().ReadOnly = false
			// A library holds image data only, which is never executed.
			res.TypedSpec().Secure = true
			res.TypedSpec().NoExec = true

			return nil
		},
	); err != nil {
		return fmt.Errorf("failed to write mount request %q: %w", requestID, err)
	}

	return nil
}

// setMountFinalizer adds the finalizer which keeps the backing volume mounted, unless the mount cannot be
// used, in which case the reason it cannot is returned instead.
func (ctrl *ContentLibraryController) setMountFinalizer(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	libraryID, volumeID string,
	volumeMountStatus *block.VolumeMountStatus,
) (string, string, error) {
	requestID := volumeMountStatus.Metadata().ID()

	if volumeMountStatus.Metadata().Phase() != resource.PhaseRunning {
		// Either the volume is going away, or this status is left over from a previous generation and
		// is still tearing down. Adding a finalizer now would block that teardown forever, so the hold
		// is dropped instead and the library reported not ready.
		if volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
			if err := runtime.RemoveFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
				return "", "", fmt.Errorf("failed to remove finalizer on %q: %w", requestID, err)
			}
		}

		return "", "the backing volume is being unmounted", nil
	}

	if volumeMountStatus.TypedSpec().ReadOnly {
		// Mount requests are merged per volume and end up read-only if every requester asked for
		// read-only, so another holder can leave this one with less access than it asked for. A
		// volume declared read-only never reaches here, which leaves the window before the block
		// subsystem settles the merged request, and requesters from outside the machine
		// configuration. Either way every upload would fail, so it is better reported here.
		return "", "the backing volume is mounted read-only", nil
	}

	if !volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := runtime.AddFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
			return "", "", fmt.Errorf("failed to add finalizer on %q: %w", requestID, err)
		}

		logger.Info("holding the volume mount for the content library",
			zap.String("library", libraryID),
			zap.String("volume", volumeID),
			zap.String("target", volumeMountStatus.TypedSpec().Target),
		)
	}

	return volumeMountStatus.TypedSpec().Target, "", nil
}

// releaseUnwanted gives back the mounts no configured library needs any more.
func (ctrl *ContentLibraryController) releaseUnwanted(
	ctx context.Context,
	runtime controller.Runtime,
	logger *zap.Logger,
	wanted map[string]struct{},
) error {
	volumeMountRequests, err := safe.ReaderListAll[*block.VolumeMountRequest](ctx, runtime)
	if err != nil {
		return fmt.Errorf("failed to list mount requests: %w", err)
	}

	for volumeMountRequest := range volumeMountRequests.All() {
		if volumeMountRequest.Metadata().Owner() != ctrl.Name() {
			continue
		}

		requestID := volumeMountRequest.Metadata().ID()

		if _, stillWanted := wanted[requestID]; stillWanted {
			continue
		}

		if err := ctrl.release(ctx, runtime, logger, requestID); err != nil {
			return err
		}
	}

	return nil
}

// release gives back one mount: the finalizer first, then the request itself.
func (ctrl *ContentLibraryController) release(ctx context.Context, runtime controller.Runtime, logger *zap.Logger, requestID string) error {
	if err := ctrl.releaseFinalizer(ctx, runtime, logger, requestID); err != nil {
		return err
	}

	return ctrl.destroyRequest(ctx, runtime, logger, requestID)
}

// releaseFinalizer drops this controller's hold on the mount status, if it holds one.
func (ctrl *ContentLibraryController) releaseFinalizer(ctx context.Context, runtime controller.Runtime, logger *zap.Logger, requestID string) error {
	volumeMountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, runtime, requestID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get mount status %q: %w", requestID, err)
	}

	if !volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	if err := runtime.RemoveFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("failed to remove finalizer on %q: %w", requestID, err)
	}

	logger.Info("released the volume mount of a content library", zap.String("request", requestID))

	return nil
}

// destroyRequest tears down the mount request and destroys it once nothing holds it.
func (ctrl *ContentLibraryController) destroyRequest(ctx context.Context, runtime controller.Runtime, logger *zap.Logger, requestID string) error {
	requestMD := block.NewVolumeMountRequest(block.NamespaceName, requestID).Metadata()

	okToDestroy, err := runtime.Teardown(ctx, requestMD)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to tear down mount request %q: %w", requestID, err)
	}

	if !okToDestroy {
		// The destroy-ready input wakes this controller once the request is free.
		logger.Debug("waiting for the volume mount request to be released", zap.String("request", requestID))

		return nil
	}

	if err := runtime.Destroy(ctx, requestMD); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to destroy mount request %q: %w", requestID, err)
	}

	return nil
}

// backingVolume is the volume a content library's backing volume name resolves to.
type backingVolume struct {
	// id is what the block subsystem knows the volume by.
	id string
	// readOnly is the mount policy the volume was declared with.
	readOnly bool
}

// resolveBackingVolume maps a content library's backing volume name to the volume behind it.
//
// The name is what the user declared the volume under; the document kind declaring it decides the
// prefix and carries the mount policy. Volume names are unique across the kinds mounted at
// `/var/mnt/<name>`, so at most one can match.
func resolveBackingVolume(cfg configcfg.Config, volumeName string) (backingVolume, bool) {
	for _, userVolumeConfig := range cfg.UserVolumeConfigs() {
		if userVolumeConfig.Name() == volumeName {
			// A user volume is never declared read-only.
			return backingVolume{id: constants.UserVolumePrefix + volumeName}, true
		}
	}

	for _, existingVolumeConfig := range cfg.ExistingVolumeConfigs() {
		if existingVolumeConfig.Name() == volumeName {
			return backingVolume{
				id:       constants.ExistingVolumePrefix + volumeName,
				readOnly: existingVolumeConfig.Mount().ReadOnly(),
			}, true
		}
	}

	for _, externalVolumeConfig := range cfg.ExternalVolumeConfigs() {
		if externalVolumeConfig.Name() == volumeName {
			return backingVolume{
				id:       constants.ExternalVolumePrefix + volumeName,
				readOnly: externalVolumeConfig.Mount().ReadOnly(),
			}, true
		}
	}

	return backingVolume{}, false
}

// sweepStagedUploads removes what interrupted uploads left staged in a library.
//
// Staged names are dot-prefixed, so they are invisible to List and unaddressable by Delete: the
// content library API cannot clear them, which is why this has to happen here. The whole mount
// target is swept, which configuration validation makes safe by letting a volume back one library
// only, and only on the transition to ready, since the API refuses a library which is not ready and
// so nothing can be staged at that moment.
func (ctrl *ContentLibraryController) sweepStagedUploads(logger *zap.Logger, libraryID, path string) {
	entries, err := os.ReadDir(path)
	if err != nil {
		logger.Warn("failed to sweep the content library", zap.String("library", libraryID), zap.Error(err))

		return
	}

	for _, entry := range entries {
		name := entry.Name()

		if !staging.IsName(name) {
			continue
		}

		if err := os.Remove(filepath.Join(path, name)); err != nil {
			logger.Warn("failed to remove a staged upload", zap.String("library", libraryID), zap.String("name", name), zap.Error(err))

			continue
		}

		logger.Info("removed a staged upload left behind by an interrupted upload",
			zap.String("library", libraryID),
			zap.String("name", name),
		)
	}
}
