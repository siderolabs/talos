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
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/selinux"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/xfs"
)

// EtcFileController watches EtcFileSpecs, creates/updates files.
type EtcFileController struct {
	// EtcRoot is the root for /etc filesystem operations.
	EtcRoot xfs.Root
}

// Name implements controller.Controller interface.
func (ctrl *EtcFileController) Name() string {
	return "files.EtcFileController"
}

// Inputs implements controller.Controller interface.
func (ctrl *EtcFileController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: files.NamespaceName,
			Type:      files.EtcFileSpecType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *EtcFileController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.EtcFileStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *EtcFileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		list, err := safe.ReaderList[*files.EtcFileSpec](ctx, r, resource.NewMetadata(files.NamespaceName, files.EtcFileSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing specs: %w", err)
		}

		// add finalizers for all live resources
		for res := range list.All() {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		touchedIDs := make(map[resource.ID]struct{})

		for spec := range list.All() {
			filename := spec.Metadata().ID()

			switch spec.Metadata().Phase() {
			case resource.PhaseTearingDown:
				logger.Debug("removing file", zap.String("name", filename))

				if err = xfs.Remove(ctrl.EtcRoot, filename); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to remove %q: %w", filename, err)
				}

				// now remove finalizer as the file was deleted
				if err = r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer: %w", err)
				}
			case resource.PhaseRunning:
				logger.Debug("writing file contents", zap.String("name", filename), zap.Stringer("version", spec.Metadata().Version()))

				if err = UpdateFile(ctrl.EtcRoot, filename, spec.TypedSpec().Contents, spec.TypedSpec().Mode, spec.TypedSpec().SelinuxLabel); err != nil {
					return fmt.Errorf("error updating %q: %w", filename, err)
				}

				if err = safe.WriterModify(ctx, r, files.NewEtcFileStatus(files.NamespaceName, filename), func(r *files.EtcFileStatus) error {
					r.TypedSpec().SpecVersion = spec.Metadata().Version().String()

					return nil
				}); err != nil {
					return fmt.Errorf("error updating status: %w", err)
				}

				touchedIDs[filename] = struct{}{}
			}
		}

		// list statuses for cleanup
		statuses, err := safe.ReaderList[*files.EtcFileStatus](ctx, r, resource.NewMetadata(files.NamespaceName, files.EtcFileStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for res := range statuses.All() {
			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}

		r.ResetRestartBackoff()
	}
}

// UpdateFile will only update the file if the contents have changed.
//
// The file is updated atomically by writing to a temporary file in the same
// directory and renaming it into place, so that concurrent readers never
// observe a truncated (empty) file while it is being rewritten.
func UpdateFile(root xfs.Root, filename string, contents []byte, mode os.FileMode, selinuxLabel string) error {
	oldContents, err := xfs.ReadFile(root, filename)
	if err == nil && bytes.Equal(oldContents, contents) {
		return nil
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
