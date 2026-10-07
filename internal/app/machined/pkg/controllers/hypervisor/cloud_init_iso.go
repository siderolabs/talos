// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/cloudinit"
	"github.com/siderolabs/talos/internal/pkg/contentlibrary/staging"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const cloudInitOwners = ".cloud-init-owners"

// CloudInitISOController builds immutable NoCloud assets on mounted content libraries.
// State is the uncached node state shared with content-library API mutation claims.
type CloudInitISOController struct{ State state.State }

func (ctrl *CloudInitISOController) Name() string {
	return "hypervisor.CloudInitISOController"
}

func (ctrl *CloudInitISOController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitSpecType,
			Kind:      controller.InputWeak,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.ContentLibraryStatusType,
			Kind:      controller.InputStrong,
		},
		{
			Namespace: hypervisor.NamespaceName,
			Type:      hypervisor.CloudInitStatusType,
			Kind:      controller.InputDestroyReady,
		},
	}
}

func (ctrl *CloudInitISOController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: hypervisor.CloudInitStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

func (ctrl *CloudInitISOController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r); err != nil {
			logger.Error("failed to reconcile cloud-init ISOs", zap.Error(err))

			return err // COSI restarts with backoff even when no further input event occurs.
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *CloudInitISOController) reconcile(ctx context.Context, r controller.ReaderWriter) error {
	specs, err := safe.ReaderListAll[*hypervisor.CloudInitSpec](ctx, r)
	if err != nil {
		return fmt.Errorf("list cloud-init specs: %w", err)
	}

	libraries, err := safe.ReaderListAll[*hypervisor.ContentLibraryStatus](ctx, r)
	if err != nil {
		return fmt.Errorf("list content libraries: %w", err)
	}

	byLibrary := make(map[string]*hypervisor.ContentLibraryStatus, libraries.Len())
	for library := range libraries.All() {
		byLibrary[library.Metadata().ID()] = library
	}

	wanted := map[resource.ID]struct{}{}

	var errs []error

	for spec := range specs.All() {
		if spec.Metadata().Phase() != resource.PhaseRunning {
			continue
		}

		id := hypervisor.CloudInitStatusID(spec.Metadata().ID(), *spec.TypedSpec())
		wanted[id] = struct{}{}
		errs = append(errs, ctrl.produceSpec(ctx, r, spec, id, byLibrary[spec.TypedSpec().Library])...)
	}

	statuses, err := safe.ReaderListAll[*hypervisor.CloudInitStatus](ctx, r)
	if err != nil {
		return errors.Join(append(errs, err)...)
	}

	for status := range statuses.All() {
		errs = append(errs, ctrl.retireStatus(ctx, r, status, wanted, byLibrary)...)
	}

	return errors.Join(append(errs, ctrl.releaseUnusedLibraries(ctx, r, byLibrary)...)...)
}

func (ctrl *CloudInitISOController) retireStatus(
	ctx context.Context, r controller.ReaderWriter, status *hypervisor.CloudInitStatus,
	wanted map[resource.ID]struct{}, byLibrary map[string]*hypervisor.ContentLibraryStatus,
) []error {
	if _, ok := wanted[status.Metadata().ID()]; ok && status.Metadata().Phase() == resource.PhaseRunning {
		return nil
	}

	if err := ctrl.retire(ctx, r, status, byLibrary[status.TypedSpec().Library]); err != nil {
		return []error{fmt.Errorf("retire cloud-init %q: %w", status.Metadata().ID(), err)}
	}

	return nil
}

func (ctrl *CloudInitISOController) produceSpec(ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id string, library *hypervisor.ContentLibraryStatus) []error {
	if err := ctrl.produce(ctx, r, spec, id, library); err != nil {
		var errs []error

		if statusErr := ctrl.publicationError(ctx, r, id, err); statusErr != nil {
			errs = append(errs, fmt.Errorf("publish cloud-init %q error status: %w", id, statusErr))
		}

		return append(errs, fmt.Errorf("cloud-init %q: %w", spec.Metadata().ID(), err))
	}

	return nil
}

func (ctrl *CloudInitISOController) releaseUnusedLibraries(ctx context.Context, r controller.ReaderWriter, byLibrary map[string]*hypervisor.ContentLibraryStatus) []error {
	statuses, err := safe.ReaderListAll[*hypervisor.CloudInitStatus](ctx, r)
	if err != nil {
		return []error{err}
	}

	inUse := map[string]struct{}{}

	for status := range statuses.All() {
		if status.TypedSpec().Path != "" {
			inUse[status.TypedSpec().Library] = struct{}{}
		}
	}

	var errs []error

	for id, library := range byLibrary {
		if _, ok := inUse[id]; ok || !library.Metadata().Finalizers().Has(ctrl.Name()) {
			continue
		}

		if err := r.RemoveFinalizer(ctx, library.Metadata(), ctrl.Name()); err != nil && !state.IsNotFoundError(err) {
			errs = append(errs, fmt.Errorf("release library %q: %w", id, err))
		}
	}

	return errs
}

func (ctrl *CloudInitISOController) publicationError(ctx context.Context, r controller.ReaderWriter, id string, cause error) error {
	status, err := safe.ReaderGetByID[*hypervisor.CloudInitStatus](ctx, r, id)
	if state.IsNotFoundError(err) {
		return nil // Reservation failed; do not create an unowned status here.
	}

	if err != nil {
		return err
	}

	if status.Metadata().Phase() != resource.PhaseRunning || status.TypedSpec().Ready {
		return nil
	}

	reason := "cloud-init ISO publication failed; retrying"
	if errors.Is(cause, fs.ErrExist) {
		reason = "cloud-init ISO name is occupied; refusing to replace another file"
	}

	return safe.WriterModify(ctx, r, hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id), func(current *hypervisor.CloudInitStatus) error {
		if !current.TypedSpec().Ready {
			current.TypedSpec().Error = reason
		}

		return nil
	})
}

func (ctrl *CloudInitISOController) waiting(ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id, reason string) error {
	return safe.WriterModify(ctx, r, hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id), func(status *hypervisor.CloudInitStatus) error {
		if status.TypedSpec().Path != "" || status.Metadata().Phase() != resource.PhaseRunning {
			return nil // Never rewrite a published or held incarnation.
		}

		*status.TypedSpec() = hypervisor.CloudInitStatusSpec{
			VirtualMachine:     spec.Metadata().ID(),
			Library:            spec.TypedSpec().Library,
			InputDigest:        spec.TypedSpec().InputDigest(),
			ObservedGeneration: spec.Metadata().Version().String(),
			Error:              reason,
		}

		return nil
	})
}

func cloudInitLibraryReady(library *hypervisor.ContentLibraryStatus) bool {
	return library.TypedSpec().Ready && library.Metadata().Phase() == resource.PhaseRunning
}

func (ctrl *CloudInitISOController) handleExistingISO(
	ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id string,
	old *hypervisor.CloudInitStatus, library *hypervisor.ContentLibraryStatus,
) (bool, error) {
	if library == nil || old.TypedSpec().Path != library.TypedSpec().Path || old.TypedSpec().VolumeID != library.TypedSpec().VolumeID {
		return true, ctrl.retire(ctx, r, old, library)
	}

	if old.TypedSpec().InputDigest != spec.TypedSpec().InputDigest() || old.TypedSpec().ObservedGeneration != spec.Metadata().Version().String() {
		return true, ctrl.retire(ctx, r, old, library)
	}

	if !old.TypedSpec().Ready {
		// Resume a previously linked asset only when its durable witness proves ownership.
		return false, nil
	}

	if cloudInitLibraryReady(library) {
		if err := checkOwnedISO(old.TypedSpec()); err == nil {
			return true, nil
		}
	}

	// A finalizer prevents destruction, not modification. Retire a held status
	// instead of repointing it to different bytes.
	if err := safe.WriterModify(ctx, r, hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id), func(status *hypervisor.CloudInitStatus) error {
		status.TypedSpec().Ready = false
		status.TypedSpec().Error = "cloud-init ISO is missing or changed"

		return nil
	}); err != nil {
		return true, err
	}

	return true, ctrl.retire(ctx, r, old, library)
}

func (ctrl *CloudInitISOController) produce(ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id string, library *hypervisor.ContentLibraryStatus) error {
	finished, old, err := ctrl.prepareISO(ctx, r, spec, id, library)
	if finished || err != nil {
		return err
	}

	current, name, err := ctrl.reserveISO(ctx, r, spec, id, library, old)
	if err != nil || current == nil {
		return err
	}

	return ctrl.produceReservedISO(ctx, r, spec, id, current, name)
}

func (ctrl *CloudInitISOController) prepareISO(
	ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id string,
	library *hypervisor.ContentLibraryStatus,
) (bool, *hypervisor.CloudInitStatus, error) {
	seed := spec.TypedSpec()

	if reason := invalidSeedReason(seed); reason != "" {
		return true, nil, ctrl.waiting(ctx, r, spec, id, reason)
	}

	old, err := safe.ReaderGetByID[*hypervisor.CloudInitStatus](ctx, r, id)
	if err != nil && !state.IsNotFoundError(err) {
		return true, nil, err
	}

	if old != nil && old.Metadata().Phase() != resource.PhaseRunning {
		return true, old, nil
	}

	if old != nil && old.TypedSpec().Path != "" {
		finished, oldErr := ctrl.handleExistingISO(ctx, r, spec, id, old, library)
		if finished {
			return true, old, oldErr
		}
	}

	if reason := unavailableLibraryReason(library); reason != "" {
		return true, old, ctrl.waiting(ctx, r, spec, id, reason)
	}

	return false, old, nil
}

func invalidSeedReason(seed *hypervisor.CloudInitSpecSpec) string {
	if seed.Library == "" || strings.ContainsAny(seed.Library, "/\\\x00") {
		return "invalid content library name"
	}

	if len(seed.MetaData)+len(seed.UserData)+len(seed.NetworkConfig) > cloudinit.MaxSeedBytes {
		return "cloud-init seed exceeds size limit"
	}

	return ""
}

func unavailableLibraryReason(library *hypervisor.ContentLibraryStatus) string {
	if library == nil {
		return "content library is not configured"
	}

	if !library.TypedSpec().Ready || library.Metadata().Phase() != resource.PhaseRunning || library.TypedSpec().Path == "" {
		return "waiting for content library to become ready"
	}

	return ""
}

func (ctrl *CloudInitISOController) produceReservedISO(
	ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id string,
	current *hypervisor.ContentLibraryStatus, name string,
) (err error) {
	seed := spec.TypedSpec()
	path := current.TypedSpec().Path

	root, err := os.OpenRoot(path)
	if err != nil {
		return fmt.Errorf("open library: %w", err)
	}
	defer root.Close() //nolint:errcheck

	unlock, err := ctrl.claimFile(ctx, seed.Library, name, false)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, unlock()) }()

	if err := root.Mkdir(cloudInitOwners, 0o700); err != nil && !errors.Is(err, fs.ErrExist) {
		return fmt.Errorf("create ISO ownership directory: %w", err)
	}

	witness := filepath.Join(cloudInitOwners, name)

	if err := ctrl.publishISO(ctx, root, path, name, witness, seed); err != nil {
		return err
	}

	return ctrl.completeISO(ctx, r, spec, id, current, root, name)
}

func (ctrl *CloudInitISOController) publishISO(ctx context.Context, root *os.Root, path, name, witness string, seed *hypervisor.CloudInitSpecSpec) error {
	owned, err := sameInode(root, witness, name)
	if err != nil {
		return err
	}

	if owned {
		return nil
	}

	if _, statErr := root.Lstat(name); statErr == nil {
		return fmt.Errorf("refusing foreign cloud-init ISO collision %q: %w", name, fs.ErrExist)
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}

	if info, statErr := root.Lstat(witness); statErr == nil {
		if !info.Mode().IsRegular() {
			return fmt.Errorf("refusing non-file cloud-init ownership witness %q", name)
		}
		// Recovery from a crash after the witness was synced but before the
		// final hard link. This witness was created only after status reservation.
		if err := root.Link(witness, name); err != nil {
			return fmt.Errorf("recover cloud-init ISO link: %w", err)
		}

		if err := syncDirectory(root, "."); err != nil {
			return err
		}

		return nil
	} else if !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}

	return writeAndLinkISO(ctx, root, path, name, witness, seed)
}

func writeAndLinkISO(ctx context.Context, root *os.Root, path, name, witness string, seed *hypervisor.CloudInitSpecSpec) error {
	stage := staging.Name(name)
	if err := cloudinit.WriteISO(ctx, filepath.Join(path, stage), cloudinit.Seed{
		MetaData:      seed.MetaData,
		UserData:      seed.UserData,
		NetworkConfig: seed.NetworkConfig,
	}); err != nil {
		return err
	}
	defer root.Remove(stage) //nolint:errcheck // staging is private; recovery never adopts it

	if err := ctx.Err(); err != nil {
		return err
	}

	if err := root.Link(stage, witness); err != nil {
		return fmt.Errorf("record ISO ownership: %w", err)
	}

	if err := syncDirectory(root, cloudInitOwners); err != nil {
		return err
	}

	if err := root.Link(stage, name); err != nil {
		return fmt.Errorf("publish ISO without overwrite: %w", err)
	}

	if err := syncDirectory(root, "."); err != nil {
		return err
	}

	return nil
}

func (ctrl *CloudInitISOController) reserveISO(
	ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id string,
	library *hypervisor.ContentLibraryStatus, old *hypervisor.CloudInitStatus,
) (*hypervisor.ContentLibraryStatus, string, error) {
	if !library.Metadata().Finalizers().Has(ctrl.Name()) {
		if err := r.AddFinalizer(ctx, library.Metadata(), ctrl.Name()); err != nil {
			return nil, "", fmt.Errorf("hold library: %w", err)
		}
	}

	current, err := safe.ReaderGetByID[*hypervisor.ContentLibraryStatus](ctx, r, spec.TypedSpec().Library)
	if err != nil {
		return nil, "", err
	}

	if !sameReadyLibraryBacking(current, library) {
		return nil, "", ctrl.waiting(ctx, r, spec, id, "content library backing changed; waiting for a stable mount")
	}

	name := "cloud-init-" + id[strings.LastIndex(id, "@")+1:] + ".iso"

	// Reserve status before any durable file. The witness hard link proves inode
	// ownership on restart; matching bytes alone cannot adopt a foreign file.
	if old == nil || old.TypedSpec().Name == "" {
		if err := safe.WriterModify(ctx, r, hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id), func(status *hypervisor.CloudInitStatus) error {
			*status.TypedSpec() = hypervisor.CloudInitStatusSpec{
				VirtualMachine:     spec.Metadata().ID(),
				Library:            spec.TypedSpec().Library,
				Name:               name,
				Path:               current.TypedSpec().Path,
				VolumeID:           current.TypedSpec().VolumeID,
				InputDigest:        spec.TypedSpec().InputDigest(),
				ObservedGeneration: spec.Metadata().Version().String(),
				Error:              "building cloud-init ISO",
			}

			return nil
		}); err != nil {
			return nil, "", fmt.Errorf("reserve cloud-init status: %w", err)
		}
	}

	return current, name, nil
}

func sameReadyLibraryBacking(current, library *hypervisor.ContentLibraryStatus) bool {
	return current.TypedSpec().Ready && current.Metadata().Phase() == resource.PhaseRunning &&
		current.TypedSpec().Path == library.TypedSpec().Path && current.TypedSpec().VolumeID == library.TypedSpec().VolumeID
}

func (ctrl *CloudInitISOController) completeISO(
	ctx context.Context, r controller.ReaderWriter, spec *hypervisor.CloudInitSpec, id string,
	library *hypervisor.ContentLibraryStatus, root *os.Root, name string,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	file, err := root.Open(name)
	if err != nil {
		return err
	}

	digest, size, hashErr := hashISO(file)

	closeErr := file.Close()
	if err := errors.Join(hashErr, closeErr); err != nil {
		return err
	}

	return safe.WriterModify(ctx, r, hypervisor.NewCloudInitStatus(hypervisor.NamespaceName, id), func(status *hypervisor.CloudInitStatus) error {
		if status.TypedSpec().Path != library.TypedSpec().Path || status.TypedSpec().Name != name ||
			status.TypedSpec().VolumeID != library.TypedSpec().VolumeID || status.TypedSpec().InputDigest != spec.TypedSpec().InputDigest() {
			return errors.New("cloud-init reservation changed during publication")
		}

		status.TypedSpec().Digest = digest
		status.TypedSpec().SizeBytes = size
		status.TypedSpec().Ready = true
		status.TypedSpec().Error = ""

		return nil
	})
}

func hashISO(file *os.File) (string, uint64, error) {
	h := sha256.New()

	n, err := io.Copy(h, file)
	if err != nil {
		return "", 0, fmt.Errorf("hash ISO: %w", err)
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), uint64(n), nil
}

func syncDirectory(root *os.Root, name string) error {
	dir, err := root.Open(name)
	if err != nil {
		return err
	}

	return errors.Join(dir.Sync(), dir.Close())
}

func sameInode(root *os.Root, witness, name string) (bool, error) {
	proof, err := root.Lstat(witness)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	asset, err := root.Lstat(name)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}

	if err != nil {
		return false, err
	}

	return proof.Mode().IsRegular() && asset.Mode().IsRegular() && os.SameFile(proof, asset), nil
}

func checkOwnedISO(spec *hypervisor.CloudInitStatusSpec) error {
	root, err := os.OpenRoot(spec.Path)
	if err != nil {
		return err
	}
	defer root.Close() //nolint:errcheck

	owned, err := sameInode(root, filepath.Join(cloudInitOwners, spec.Name), spec.Name)
	if err != nil {
		return err
	}

	if !owned {
		return errors.New("cloud-init ISO ownership changed")
	}

	file, err := root.Open(spec.Name)
	if err != nil {
		return err
	}

	digest, size, hashErr := hashISO(file)
	if err := errors.Join(hashErr, file.Close()); err != nil {
		return err
	}

	if digest != spec.Digest || size != spec.SizeBytes {
		return errors.New("cloud-init ISO bytes changed")
	}

	return nil
}

func (ctrl *CloudInitISOController) retire(ctx context.Context, r controller.ReaderWriter, status *hypervisor.CloudInitStatus, library *hypervisor.ContentLibraryStatus) (err error) {
	md := status.Metadata()

	ready, err := teardownISOStatus(ctx, r, md)
	if err != nil || !ready {
		return err
	}

	spec := status.TypedSpec()
	if spec.Path != "" && spec.Name != "" {
		// A path may have been remounted under the same spelling.
		if !pinnedLibraryMatches(spec, library) {
			return fmt.Errorf("old cloud-init library backing %q is unavailable; retaining status and hold", spec.Library)
		}

		unlock, claimErr := ctrl.claimFile(ctx, spec.Library, spec.Name, true)
		if claimErr != nil {
			return claimErr
		}
		defer func() { err = errors.Join(err, unlock()) }()

		if err := removeRetiredISO(spec); err != nil {
			return err
		}
	}

	if err := r.Destroy(ctx, md); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("destroy cloud-init status %q: %w", md.ID(), err)
	}

	return nil
}

func pinnedLibraryMatches(spec *hypervisor.CloudInitStatusSpec, library *hypervisor.ContentLibraryStatus) bool {
	return library != nil && library.TypedSpec().Path == spec.Path && library.TypedSpec().VolumeID == spec.VolumeID
}

func teardownISOStatus(ctx context.Context, r controller.ReaderWriter, md *resource.Metadata) (bool, error) {
	ready, err := r.Teardown(ctx, md)
	if err != nil && !state.IsNotFoundError(err) {
		return false, fmt.Errorf("teardown cloud-init status %q: %w", md.ID(), err)
	}

	return ready, nil
}

func removeRetiredISO(spec *hypervisor.CloudInitStatusSpec) error {
	root, err := os.OpenRoot(spec.Path)
	if err != nil {
		return fmt.Errorf("open old ISO library: %w", err)
	}
	defer root.Close() //nolint:errcheck

	witness := filepath.Join(cloudInitOwners, spec.Name)
	if err := removeOwnedISO(root, spec, witness); err != nil {
		return err
	}

	if err := root.Remove(witness); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete ISO ownership witness: %w", err)
	}

	return syncDirectory(root, cloudInitOwners)
}

func removeOwnedISO(root *os.Root, spec *hypervisor.CloudInitStatusSpec, witness string) error {
	owned, err := sameInode(root, witness, spec.Name)
	if err != nil {
		return err
	}

	if !owned {
		if _, statErr := root.Lstat(spec.Name); statErr == nil {
			// An unrelated file occupies our old name. Leave it untouched.
			return fmt.Errorf("refusing to delete foreign ISO %q", spec.Name)
		} else if !errors.Is(statErr, fs.ErrNotExist) {
			return statErr
		}

		return nil
	}

	if spec.Digest != "" {
		if err := checkOwnedISO(spec); err != nil {
			return fmt.Errorf("refusing to delete changed ISO %q: %w", spec.Name, err)
		}
	}

	if err := root.Remove(spec.Name); err != nil {
		return fmt.Errorf("delete ISO %q: %w", spec.Name, err)
	}

	return syncDirectory(root, ".")
}

// claimFile shares the API's atomic per-file mutation marker. Once teardown
// has started, the API's PhaseRunning claim cannot enter, so cleanup only has
// to wait for a marker acquired before teardown. The controller's strong hold
// keeps the mount alive until cleanup is done.
func (ctrl *CloudInitISOController) claimFile(ctx context.Context, library, name string, retiring bool) (func() error, error) {
	md := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, library).Metadata()
	marker := hypervisor.ContentLibraryMutationFinalizer(name)
	locked := errors.New("cloud-init asset is being mutated")

	for {
		current, err := ctrl.State.Get(ctx, md)
		if err != nil {
			return nil, err
		}

		if retiring && current.Metadata().Phase() == resource.PhaseTearingDown {
			if current.Metadata().Finalizers().Has(marker) {
				return nil, locked
			}

			// A prior claim would still be visible. Teardown bars new API claims.
			return func() error { return nil }, nil
		}

		err = ctrl.acquireFileMarker(ctx, md, current.Metadata().Owner(), marker, locked)
		if retiring && state.IsPhaseConflictError(err) {
			continue // Teardown won; re-read its pre-existing claims.
		}

		if err != nil {
			return nil, err
		}

		break
	}

	return func() error {
		return ctrl.releaseFileMarker(ctx, md, marker, name)
	}, nil
}

func (ctrl *CloudInitISOController) releaseFileMarker(ctx context.Context, md *resource.Metadata, marker, name string) error {
	// Cleanup must finish even when reconciliation is canceled, but must not
	// outlive a bounded shutdown indefinitely.
	releaseCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()

	if err := ctrl.State.RemoveFinalizer(releaseCtx, md, marker); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("release cloud-init mutation claim %q: %w", name, err)
	}

	return nil
}

func (ctrl *CloudInitISOController) acquireFileMarker(ctx context.Context, md *resource.Metadata, owner resource.Owner, marker string, locked error) error {
	_, err := ctrl.State.UpdateWithConflicts(ctx, md, func(res resource.Resource) error {
		if res.Metadata().Finalizers().Has(marker) {
			return locked
		}

		res.Metadata().Finalizers().Add(marker)

		return nil
	}, state.WithUpdateOwner(owner), state.WithExpectedPhase(resource.PhaseRunning))

	return err
}
