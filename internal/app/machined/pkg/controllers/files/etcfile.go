// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/xfs"
)

// EtcFileController watches EtcFileSpecs, creates/updates files.
//
// EtcFileController is a QController: each EtcFileSpec is reconciled on its own when it changes,
// so a change to one file doesn't cause all the other files (e.g. the large CA bundle) to be
// copied out of the state and compared to the contents on disk.
type EtcFileController struct {
	// EtcRoot is the root for /etc filesystem operations.
	EtcRoot xfs.Root
}

// Name implements controller.QController interface.
func (ctrl *EtcFileController) Name() string {
	return "files.EtcFileController"
}

// Settings implements controller.QController interface.
func (ctrl *EtcFileController) Settings() controller.QSettings {
	return controller.QSettings{
		Inputs: []controller.Input{
			{
				Namespace: files.NamespaceName,
				Type:      files.EtcFileSpecType,
				Kind:      controller.InputQPrimary,
			},
		},
		Outputs: []controller.Output{
			{
				Type: files.EtcFileStatusType,
				Kind: controller.OutputExclusive,
			},
		},
	}
}

// MapInput implements controller.QController interface.
func (ctrl *EtcFileController) MapInput(_ context.Context, _ *zap.Logger, _ controller.QRuntime, ptr controller.ReducedResourceMetadata) ([]resource.Pointer, error) {
	return nil, fmt.Errorf("unexpected input %s", ptr)
}

// Reconcile implements controller.QController interface.
func (ctrl *EtcFileController) Reconcile(ctx context.Context, logger *zap.Logger, r controller.QRuntime, ptr resource.Pointer) error {
	spec, err := safe.ReaderGet[*files.EtcFileSpec](ctx, r, ptr)
	if err != nil {
		if state.IsNotFoundError(err) {
			// the spec is gone without the finalizer being set (it was never written), clean up the status if any
			return ctrl.destroyStatus(ctx, r, ptr.ID())
		}

		return fmt.Errorf("error getting spec: %w", err)
	}

	switch spec.Metadata().Phase() {
	case resource.PhaseTearingDown:
		return ctrl.teardown(ctx, logger, r, spec)
	case resource.PhaseRunning:
		return ctrl.write(ctx, logger, r, spec)
	default:
		return fmt.Errorf("unexpected phase %s", spec.Metadata().Phase())
	}
}

func (ctrl *EtcFileController) teardown(ctx context.Context, logger *zap.Logger, r controller.QRuntime, spec *files.EtcFileSpec) error {
	filename := spec.Metadata().ID()

	if err := ctrl.destroyStatus(ctx, r, filename); err != nil {
		return err
	}

	if !spec.Metadata().Finalizers().Has(ctrl.Name()) {
		// the file was never written
		return nil
	}

	logger.Debug("removing file", zap.String("name", filename))

	if err := xfs.Remove(ctrl.EtcRoot, filename); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("failed to remove %q: %w", filename, err)
	}

	// now remove finalizer as the file was deleted
	if err := r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
		return fmt.Errorf("error removing finalizer: %w", err)
	}

	return nil
}

func (ctrl *EtcFileController) write(ctx context.Context, logger *zap.Logger, r controller.QRuntime, spec *files.EtcFileSpec) error {
	filename := spec.Metadata().ID()

	if !spec.Metadata().Finalizers().Has(ctrl.Name()) {
		// adding the finalizer bumps the spec version, which requeues the spec,
		// so the file is written (once) with the final version on the next reconcile
		if err := r.AddFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
			return fmt.Errorf("error adding finalizer: %w", err)
		}

		return nil
	}

	logger.Debug("writing file contents", zap.String("name", filename), zap.Stringer("version", spec.Metadata().Version()))

	if err := AtomicUpdate(ctrl.EtcRoot, filename, spec.TypedSpec().Contents, spec.TypedSpec().Mode, spec.TypedSpec().SelinuxLabel); err != nil {
		return fmt.Errorf("error updating %q: %w", filename, err)
	}

	if err := safe.WriterModify(ctx, r, files.NewEtcFileStatus(files.NamespaceName, filename), func(status *files.EtcFileStatus) error {
		status.TypedSpec().SpecVersion = spec.Metadata().Version().String()

		return nil
	}); err != nil {
		return fmt.Errorf("error updating status: %w", err)
	}

	return nil
}

func (ctrl *EtcFileController) destroyStatus(ctx context.Context, r controller.QRuntime, id resource.ID) error {
	if err := r.Destroy(ctx, files.NewEtcFileStatus(files.NamespaceName, id).Metadata()); err != nil && !state.IsNotFoundError(err) {
		return fmt.Errorf("error cleaning up status: %w", err)
	}

	return nil
}

// UpdateFile is like `os.WriteFile`, but it will only update the file if the
// contents have changed.
//
// The file is updated in place (truncate + write), which preserves the file's
// inode. This is required for files that are bind-mounted by inode (e.g. the pod
// resolv.conf, see network.EtcFileController), where a rename would orphan the
// bind mount. Callers that do not have this constraint should prefer AtomicUpdate.
func UpdateFile(root xfs.Root, filename string, contents []byte, mode os.FileMode, selinuxLabel string) error {
	oldContents, err := xfs.ReadFile(root, filename)
	if err == nil && bytes.Equal(oldContents, contents) {
		return selinux.FSetLabel(root, filename, selinuxLabel)
	}

	if err = xfs.MkdirAll(root, filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("mkdir all failed: %w", err)
	}

	if err := xfs.WriteFile(root, filename, contents, mode); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	return selinux.FSetLabel(root, filename, selinuxLabel)
}

// AtomicUpdate is like UpdateFile, but it updates the file atomically by writing
// to a temporary file in the same directory and renaming it into place, so that
// concurrent readers never observe a truncated (empty) file while it is being
// rewritten.
//
// The rename replaces the file's inode, so this MUST NOT be used for files that
// are bind-mounted by inode (use UpdateFile for those).
func AtomicUpdate(root xfs.Root, filename string, contents []byte, mode os.FileMode, selinuxLabel string) error {
	oldContents, err := xfs.ReadFile(root, filename)
	if err == nil && bytes.Equal(oldContents, contents) {
		return selinux.FSetLabel(root, filename, selinuxLabel)
	}

	if err = xfs.MkdirAll(root, filepath.Dir(filename), 0o755); err != nil {
		return fmt.Errorf("mkdir all failed: %w", err)
	}

	tmpFilename := filename + ".tmp"

	if err := xfs.WriteFile(root, tmpFilename, contents, mode); err != nil {
		return fmt.Errorf("write file failed: %w", err)
	}

	defer xfs.Remove(root, tmpFilename) //nolint:errcheck

	// label the temporary file before the rename so the final file appears
	// atomically with the correct SELinux label.
	if err := selinux.FSetLabel(root, tmpFilename, selinuxLabel); err != nil {
		return err
	}

	if err := xfs.Rename(root, tmpFilename, filename); err != nil {
		return fmt.Errorf("rename failed: %w", err)
	}

	return nil
}
