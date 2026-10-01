// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package hypervisor provides controllers for the Talos hypervisor.
package hypervisor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/fsnotify/fsnotify"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/panicsafe"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/contentlibrary/staging"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// contentLibraryLabel names the library a mount request was made for.
const contentLibraryLabel = "content-library"

// ContentLibraryController mounts the volume backing each configured content library.
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
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.ContentLibraryStatusType,
			Kind:      controller.InputWeak,
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
	contentsWatch, err := startContentsWatch(runtime, logger)
	if err != nil {
		return err
	}

	defer contentsWatch.stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-runtime.EventCh():
		case <-contentsWatch.failed:
			return errors.New("the content library contents watch stopped")
		}

		if err := ctrl.reconcile(ctx, runtime, logger, contentsWatch); err != nil {
			logger.Error("failed to reconcile content libraries", zap.Error(err))

			return err
		}

		runtime.ResetRestartBackoff()
	}
}

// contentsWatch reconciles the controller whenever a watched library directory changes.
//
// A library's contents are files, and writing one produces no resource event. This turns an image
// arriving, changing or being deleted into a reconciliation, which republishes the library's
// fingerprint and so wakes whoever is waiting on that file.
type contentsWatch struct {
	watcher *fsnotify.Watcher
	// watched is the set of directories the watch currently covers.
	watched map[string]struct{}
	// failed is closed once the watch is gone, so that the controller restarts.
	failed chan struct{}
	done   sync.WaitGroup
}

func startContentsWatch(runtime controller.Runtime, logger *zap.Logger) (*contentsWatch, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create fsnotify watcher: %w", err)
	}

	watch := &contentsWatch{
		watcher: watcher,
		watched: map[string]struct{}{},
		failed:  make(chan struct{}),
	}

	watch.done.Go(func() {
		defer close(watch.failed)

		err := panicsafe.RunErr(func() error {
			for {
				select {
				case _, ok := <-watcher.Events:
					if !ok {
						return nil
					}

					runtime.QueueReconcile()
				case err, ok := <-watcher.Errors:
					if !ok {
						return nil
					}

					logger.Warn("content library watch error", zap.Error(err))

					runtime.QueueReconcile()
				}
			}
		})
		if err != nil {
			logger.Error("content library watch failed", zap.Error(err))
		}
	})

	return watch, nil
}

func (w *contentsWatch) stop() {
	w.watcher.Close() //nolint:errcheck

	w.done.Wait()
}

// watch covers a directory, re-establishing the watch every time it is called: a watch follows the
// inode it was established on, not the name, so this also picks up whatever a new mount put at the
// path.
func (w *contentsWatch) watch(path string) error {
	if err := w.watcher.Add(path); err != nil {
		return fmt.Errorf("failed to watch %q: %w", path, err)
	}

	w.watched[path] = struct{}{}

	return nil
}

// prune stops watching the directories outside wanted.
func (w *contentsWatch) prune(wanted map[string]struct{}) {
	maps.DeleteFunc(w.watched, func(path string, _ struct{}) bool {
		if _, keep := wanted[path]; keep {
			return false
		}

		w.watcher.Remove(path) //nolint:errcheck // the watch is gone either way

		return true
	})
}

// FingerprintLibrary fingerprints a library's listing, so the value changes exactly when the set
// of files, their sizes or their modification times do.
//
// Uploads in flight are staged under a reserved prefix and excluded: a partial upload must not
// advertise itself as a change to the library's contents.
func FingerprintLibrary(path string) (string, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return "", err
	}

	sum := sha256.New()

	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), staging.Prefix) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				// Raced with a delete; the event that delete raised brings us back here.
				continue
			}

			return "", err
		}

		fmt.Fprintf(sum, "%s\x00%d\x00%d\x00", entry.Name(), info.Size(), info.ModTime().UnixNano())
	}

	return hex.EncodeToString(sum.Sum(nil)), nil
}

func (ctrl *ContentLibraryController) reconcile(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, watch *contentsWatch,
) error {
	machineConfig, err := safe.ReaderGetByID[*config.MachineConfig](ctx, r, config.ActiveID)
	if err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to get machine config: %w", err)
	}

	wantedMountRequests, wantedWatches, configured, err := ctrl.reconcileLibraries(ctx, r, logger, watch, machineConfig)
	if err != nil {
		return err
	}

	watch.prune(wantedWatches)

	held, err := ctrl.heldLibraries(ctx, r)
	if err != nil {
		return err
	}

	// Before the statuses are cleaned up, so that a hold still has somewhere to be dropped.
	if err := ctrl.releaseUnwanted(ctx, r, logger, wantedMountRequests, held); err != nil {
		return err
	}

	return cleanupOutputs[*hypervisor.ContentLibraryStatus](ctx, r, "content library status", configured)
}

// heldLibraries reports the libraries something outside this controller is using, by ID.
func (ctrl *ContentLibraryController) heldLibraries(ctx context.Context, reader controller.Reader) (map[string]struct{}, error) {
	libraryStatuses, err := safe.ReaderListAll[*hypervisor.ContentLibraryStatus](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to list content library statuses: %w", err)
	}

	held := map[string]struct{}{}

	for libraryStatus := range libraryStatuses.All() {
		if !libraryStatus.Metadata().Finalizers().Empty() {
			held[libraryStatus.Metadata().ID()] = struct{}{}
		}
	}

	return held, nil
}

// reconcileLibraries brings up every configured library.
func (ctrl *ContentLibraryController) reconcileLibraries(
	ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, watch *contentsWatch,
	machineConfig *config.MachineConfig,
) (mountRequests, watches, configured map[string]struct{}, err error) {
	mountRequests = map[string]struct{}{}
	watches = map[string]struct{}{}
	configured = map[string]struct{}{}

	if machineConfig == nil || machineConfig.Config() == nil {
		return mountRequests, watches, configured, nil
	}

	for _, contentLibraryConfig := range machineConfig.Config().ContentLibraryConfigs() {
		libraryID := contentLibraryConfig.Name()
		configured[libraryID] = struct{}{}

		leaving, err := isLeaving(ctx, r, libraryID)
		if err != nil {
			return nil, nil, nil, err
		}

		if leaving {
			continue
		}

		requestID, watchPath, err := ctrl.reconcileLibrary(ctx, r, logger, watch, machineConfig.Config(), contentLibraryConfig)
		if err != nil {
			return nil, nil, nil, err
		}

		if requestID != "" {
			mountRequests[requestID] = struct{}{}
		}

		if watchPath != "" {
			watches[watchPath] = struct{}{}
		}
	}

	return mountRequests, watches, configured, nil
}

// isLeaving reports whether a library's status is on its way out.
func isLeaving(ctx context.Context, reader controller.Reader, libraryID string) (bool, error) {
	existing, err := safe.ReaderGetByID[*hypervisor.ContentLibraryStatus](ctx, reader, libraryID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return false, nil
		}

		return false, fmt.Errorf("failed to get content library status %q: %w", libraryID, err)
	}

	return existing.Metadata().Phase() != resource.PhaseRunning, nil
}

// publishStatus writes one library's status and reports whether this write is the one that made it
// ready, which is the only moment nothing can be staged in it.
//
// A library that is not ready has no directory to list, so it keeps its last fingerprint.
func (ctrl *ContentLibraryController) publishStatus(
	ctx context.Context, r controller.ReaderWriter, libraryID, volumeID, path, reason string,
) (bool, error) {
	ready := reason == ""

	var fingerprint string

	if ready {
		var err error

		if fingerprint, err = FingerprintLibrary(path); err != nil {
			return false, fmt.Errorf("failed to read content library %q: %w", libraryID, err)
		}
	}

	var becameReady bool

	if err := safe.WriterModify(
		ctx, r,
		hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, libraryID),
		func(res *hypervisor.ContentLibraryStatus) error {
			becameReady = ready && !res.TypedSpec().Ready

			res.TypedSpec().VolumeID = volumeID
			res.TypedSpec().Path = path
			res.TypedSpec().Ready = ready
			res.TypedSpec().Error = reason

			if ready {
				res.TypedSpec().Fingerprint = fingerprint
			}

			return nil
		},
	); err != nil {
		return false, fmt.Errorf("failed to write content library status %q: %w", libraryID, err)
	}

	return becameReady, nil
}

// reconcileLibrary brings one content library up and reports what came of it on its status.
//
// Returns the ID of the mount request the library wants, empty when it wants none.
func (ctrl *ContentLibraryController) reconcileLibrary(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	watch *contentsWatch,
	cfg configcfg.Config,
	contentLibraryConfig configcfg.ContentLibraryConfig,
) (string, string, error) {
	libraryID := contentLibraryConfig.Name()
	volumeName := contentLibraryConfig.BackingVolumeName()

	backing, resolved := configcfg.ResolveBackingVolume(cfg, volumeName)

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
	case backing.ReadOnly:
		// Nothing is asked for: a mount is read-only only while every requester asks for it, so the
		// read-write mount a library needs would remount the volume against what the user declared.
		reason = fmt.Sprintf("backing volume %q is configured read-only", volumeName)
	default:
		requestID = ctrl.mountRequestID(libraryID, backing.ID)

		var err error

		if path, reason, err = ctrl.mount(ctx, r, logger, libraryID, backing.ID, requestID); err != nil {
			return "", "", err
		}
	}

	watchPath := ""

	if reason == "" {
		watchPath = path

		if err := watch.watch(path); err != nil {
			return "", "", fmt.Errorf("content library %q: %w", libraryID, err)
		}
	}

	becameReady, err := ctrl.publishStatus(ctx, r, libraryID, backing.ID, path, reason)
	if err != nil {
		return "", "", err
	}

	if becameReady {
		ctrl.sweepStagedUploads(logger, libraryID, path)
	}

	return requestID, watchPath, nil
}

// mountRequestID builds the ID of the mount request for one library and its backing volume.
//
// Library names are validated against ^[A-Za-z0-9-]+$, so the separator cannot occur within one and
// parseMountRequestID reads the library back out.
func (ctrl *ContentLibraryController) mountRequestID(libraryID, volumeID string) string {
	return ctrl.Name() + "/" + libraryID + "/" + volumeID
}

// parseMountRequestID reports which library a mount request built by mountRequestID belongs to.
func (ctrl *ContentLibraryController) parseMountRequestID(requestID string) (string, bool) {
	rest, ok := strings.CutPrefix(requestID, ctrl.Name()+"/")
	if !ok {
		return "", false
	}

	libraryID, volumeID, ok := strings.Cut(rest, "/")
	if !ok || libraryID == "" || volumeID == "" {
		return "", false
	}

	return libraryID, true
}

// mount asks for the backing volume to be mounted and holds the mount once it appears.
//
// Returns the path the library's contents sit at, and the reason it is not ready yet, empty when it
// is.
func (ctrl *ContentLibraryController) mount(
	ctx context.Context,
	r controller.ReaderWriter,
	logger *zap.Logger,
	libraryID, volumeID, requestID string,
) (string, string, error) {
	if err := ctrl.requestMount(ctx, r, libraryID, volumeID, requestID); err != nil {
		return "", "", err
	}

	volumeMountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, requestID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return "", "waiting for the backing volume to be mounted", nil
		}

		return "", "", fmt.Errorf("failed to get mount status %q: %w", requestID, err)
	}

	return ctrl.setMountFinalizer(ctx, r, logger, libraryID, volumeID, volumeMountStatus)
}

// requestMount asks the block subsystem to mount the backing volume.
//
// Written whether or not the volume is ready: the request is what makes it mounted once it is.
func (ctrl *ContentLibraryController) requestMount(ctx context.Context, r controller.ReaderWriter, libraryID, volumeID, requestID string) error {
	if err := safe.WriterModify(
		ctx, r,
		block.NewVolumeMountRequest(block.NamespaceName, requestID),
		func(res *block.VolumeMountRequest) error {
			res.Metadata().Labels().Set(contentLibraryLabel, libraryID)

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
	r controller.ReaderWriter,
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
			if err := r.RemoveFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
				return "", "", fmt.Errorf("failed to remove finalizer on %q: %w", requestID, err)
			}
		}

		return "", "the backing volume is being unmounted", nil
	}

	if volumeMountStatus.TypedSpec().ReadOnly {
		// A volume declared read-only never reaches here, but mount requests are merged per volume,
		// so another requester can still leave this one read-only. Every upload would fail.
		return "", "the backing volume is mounted read-only", nil
	}

	if !volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
			return "", "", fmt.Errorf("failed to add finalizer on %q: %w", requestID, err)
		}

		logger.Info(
			"holding the volume mount for the content library",
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
	r controller.ReaderWriter,
	logger *zap.Logger,
	wanted map[string]struct{},
	held map[string]struct{},
) error {
	volumeMountRequests, err := safe.ReaderListAll[*block.VolumeMountRequest](ctx, r)
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

		library, ok := ctrl.parseMountRequestID(requestID)
		if !ok {
			// Nothing identifies which library this belongs to, so releasing it could unmount one in use.
			logger.Warn(
				"not releasing a mount request this controller cannot attribute to a content library",
				zap.String("request", requestID),
			)

			continue
		}

		if _, inUse := held[library]; inUse {
			logger.Info("keeping the volume mount of a content library still in use", zap.String("request", requestID))

			continue
		}

		if err := ctrl.release(ctx, r, logger, requestID); err != nil {
			return err
		}
	}

	return nil
}

// release gives back one mount: the finalizer first, then the request itself.
func (ctrl *ContentLibraryController) release(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, requestID string) error {
	if err := ctrl.releaseFinalizer(ctx, r, logger, requestID); err != nil {
		return err
	}

	return ctrl.destroyRequest(ctx, r, logger, requestID)
}

// releaseFinalizer drops this controller's hold on the mount status, if it holds one.
func (ctrl *ContentLibraryController) releaseFinalizer(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, requestID string) error {
	volumeMountStatus, err := safe.ReaderGetByID[*block.VolumeMountStatus](ctx, r, requestID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("failed to get mount status %q: %w", requestID, err)
	}

	if !volumeMountStatus.Metadata().Finalizers().Has(ctrl.Name()) {
		return nil
	}

	if err := r.RemoveFinalizer(ctx, volumeMountStatus.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("failed to remove finalizer on %q: %w", requestID, err)
	}

	logger.Info("released the volume mount of a content library", zap.String("request", requestID))

	return nil
}

// destroyRequest tears down the mount request and destroys it once nothing holds it.
func (ctrl *ContentLibraryController) destroyRequest(ctx context.Context, r controller.ReaderWriter, logger *zap.Logger, requestID string) error {
	requestMD := block.NewVolumeMountRequest(block.NamespaceName, requestID).Metadata()

	okToDestroy, err := r.Teardown(ctx, requestMD)
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

	if err := r.Destroy(ctx, requestMD); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("failed to destroy mount request %q: %w", requestID, err)
	}

	return nil
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

		logger.Info(
			"removed a staged upload left behind by an interrupted upload",
			zap.String("library", libraryID),
			zap.String("name", name),
		)
	}
}
