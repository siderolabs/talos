// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/pkg/machinery/resources/files"
)

// EtcFileController watches EtcFileSpecs, creates/updates files.
type EtcFileController struct {
	// Path to /etc directory, read-only filesystem.
	EtcPath string
	// Shadow path where actual file will be created and bind mounted into EtcdPath.
	ShadowPath string

	// Cache of bind mounts created.
	bindMounts map[string]interface{}
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
//nolint:gocyclo,cyclop
func (ctrl *EtcFileController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	if ctrl.bindMounts == nil {
		ctrl.bindMounts = make(map[string]interface{})
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		list, err := r.List(ctx, resource.NewMetadata(files.NamespaceName, files.EtcFileSpecType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing specs: %w", err)
		}

		// add finalizers for all live resources
		for _, res := range list.Items {
			if res.Metadata().Phase() != resource.PhaseRunning {
				continue
			}

			if err = r.AddFinalizer(ctx, res.Metadata(), ctrl.Name()); err != nil {
				return fmt.Errorf("error adding finalizer: %w", err)
			}
		}

		touchedIDs := make(map[resource.ID]struct{})

		for _, item := range list.Items {
			spec := item.(*files.EtcFileSpec) //nolint:errcheck,forcetypeassert
			filename := spec.Metadata().ID()
			_, mountExists := ctrl.bindMounts[filename]

			src := filepath.Join(ctrl.ShadowPath, filename)
			dst := filepath.Join(ctrl.EtcPath, filename)

			switch spec.Metadata().Phase() {
			case resource.PhaseTearingDown:
				if mountExists {
					logger.Debug("removing bind mount", zap.String("src", src), zap.String("dst", dst))

					if err = unix.Unmount(dst, 0); err != nil && !errors.Is(err, os.ErrNotExist) {
						return fmt.Errorf("failed to unmount bind mount %q: %w", dst, err)
					}

					delete(ctrl.bindMounts, filename)
				}

				logger.Debug("removing file", zap.String("src", src))

				if err = os.Remove(src); err != nil && !errors.Is(err, os.ErrNotExist) {
					return fmt.Errorf("failed to remove %q: %w", src, err)
				}

				// now remove finalizer as the link was deleted
				if err = r.RemoveFinalizer(ctx, spec.Metadata(), ctrl.Name()); err != nil {
					return fmt.Errorf("error removing finalizer: %w", err)
				}
			case resource.PhaseRunning:
				if !mountExists {
					logger.Debug("creating bind mount", zap.String("src", src), zap.String("dst", dst))

					if err = createBindMount(src, dst, spec.TypedSpec().Mode); err != nil {
						return fmt.Errorf("failed to create shadow bind mount %q -> %q: %w", src, dst, err)
					}

					ctrl.bindMounts[filename] = struct{}{}
				}

				logger.Debug("writing file contents", zap.String("dst", dst), zap.Stringer("version", spec.Metadata().Version()))

				if err = os.WriteFile(dst, spec.TypedSpec().Contents, spec.TypedSpec().Mode); err != nil {
					return fmt.Errorf("error updating %q: %w", dst, err)
				}

				if err = r.Modify(ctx, files.NewEtcFileStatus(files.NamespaceName, filename), func(r resource.Resource) error {
					r.(*files.EtcFileStatus).TypedSpec().SpecVersion = spec.Metadata().Version().String()

					return nil
				}); err != nil {
					return fmt.Errorf("error updating status: %w", err)
				}

				touchedIDs[filename] = struct{}{}
			}
		}

		// list statuses for cleanup
		list, err = r.List(ctx, resource.NewMetadata(files.NamespaceName, files.EtcFileStatusType, "", resource.VersionUndefined))
		if err != nil {
			return fmt.Errorf("error listing resources: %w", err)
		}

		for _, res := range list.Items {
			if _, ok := touchedIDs[res.Metadata().ID()]; !ok {
				if err = r.Destroy(ctx, res.Metadata()); err != nil {
					return fmt.Errorf("error cleaning up specs: %w", err)
				}
			}
		}
	}
}

// createBindMount creates a common way to create a writable source file with a
// bind mounted destination. This is most commonly used for well known files
// under /etc that need to be adjusted during startup.
func createBindMount(src, dst string, mode os.FileMode) (err error) {
	if err = os.MkdirAll(filepath.Dir(src), 0o755); err != nil {
		return err
	}

	var f *os.File

	if f, err = os.OpenFile(src, os.O_WRONLY|os.O_CREATE, mode); err != nil {
		return err
	}

	if err = f.Close(); err != nil {
		return err
	}

	if err = unix.Mount(src, dst, "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
		return fmt.Errorf("failed to create bind mount for %s: %w", dst, err)
	}

	return nil
}
