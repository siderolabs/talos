// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block/internal/inotify"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/v1alpha1"
)

// SymlinksController provides a view of symlinks created by udevd to the blockdevices.
type SymlinksController struct{}

// Name implements controller.Controller interface.
func (ctrl *SymlinksController) Name() string {
	return "block.SymlinksController"
}

// Inputs implements controller.Controller interface.
func (ctrl *SymlinksController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: v1alpha1.NamespaceName,
			Type:      v1alpha1.ServiceType,
			ID:        optional.Some("udevd"),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *SymlinksController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: block.SymlinkType,
			Kind: controller.OutputExclusive,
		},
	}
}

const (
	baseDevDiskPath   = "/dev/disk"
	tempSymlinkPrefix = ".#"
)

// Run implements controller.Controller interface.
//
//nolint:gocyclo,cyclop
func (ctrl *SymlinksController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	// wait for udevd to be healthy & running
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		udevdService, err := safe.ReaderGetByID[*v1alpha1.Service](ctx, r, "udevd")
		if err != nil {
			if state.IsNotFoundError(err) {
				continue
			}

			return fmt.Errorf("failed to get udevd service: %w", err)
		}

		if udevdService.TypedSpec().Healthy && udevdService.TypedSpec().Running {
			break
		}
	}

	// start the inotify watcher
	inotifyWatcher, err := inotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create inotify watcher: %w", err)
	}

	defer inotifyWatcher.Close() //nolint:errcheck

	inotifyCh, inotifyErrCh := inotifyWatcher.Run()

	// build initial list of symlinks
	//
	// map of path -> destination
	detectedSymlinks := map[string]string{}

	// get list of subpaths under /dev/disk
	if err = ctrl.handleDir(logger, inotifyWatcher, detectedSymlinks, baseDevDiskPath); err != nil {
		return err
	}

	if err = ctrl.updateOutputs(ctx, r, detectedSymlinks); err != nil {
		return err
	}

	// now wait for inotify events
	for {
		select {
		case <-ctx.Done():
			return nil
		case updatedPath := <-inotifyCh:
			logger.Debug("inotify event", zap.String("path", updatedPath))

			st, err := os.Stat(updatedPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					delete(detectedSymlinks, updatedPath)
				} else {
					return fmt.Errorf("failed to stat %q: %w", updatedPath, err)
				}
			} else {
				if st.IsDir() {
					if err = ctrl.handleDir(logger, inotifyWatcher, detectedSymlinks, updatedPath); err != nil {
						return err
					}
				} else {
					dest, err := os.Readlink(updatedPath)
					if err != nil {
						if errors.Is(err, fs.ErrNotExist) || errors.Is(err, unix.EINVAL) {
							delete(detectedSymlinks, updatedPath)
						} else {
							return fmt.Errorf("failed to readlink %q: %w", updatedPath, err)
						}
					} else if !strings.HasPrefix(filepath.Base(updatedPath), tempSymlinkPrefix) {
						detectedSymlinks[updatedPath] = dest
					}
				}
			}
		case watchErr := <-inotifyErrCh:
			return fmt.Errorf("inotify watcher failed: %w", watchErr)
		}

		if err = ctrl.updateOutputs(ctx, r, detectedSymlinks); err != nil {
			return err
		}
	}
}

func (ctrl *SymlinksController) updateOutputs(ctx context.Context, r controller.Runtime, detectedSymlinks map[string]string) error {
	r.StartTrackingOutputs()

	deviceToSymlinks := map[string][]string{}

	for path, dest := range detectedSymlinks {
		devicePath := filepath.Base(dest)

		deviceToSymlinks[devicePath] = append(deviceToSymlinks[devicePath], path)
	}

	for devicePath := range deviceToSymlinks {
		slices.Sort(deviceToSymlinks[devicePath])
	}

	for devicePath, symlinks := range deviceToSymlinks {
		if err := safe.WriterModify(ctx, r, block.NewSymlink(block.NamespaceName, devicePath), func(symlink *block.Symlink) error {
			symlink.TypedSpec().Paths = symlinks

			return nil
		}); err != nil {
			return fmt.Errorf("failed to update symlink %q: %w", devicePath, err)
		}
	}

	return safe.CleanupOutputs[*block.Symlink](ctx, r)
}

//nolint:gocyclo
func (ctrl *SymlinksController) handleDir(logger *zap.Logger, inotifyWatcher *inotify.Watcher, detectedSymlinks map[string]string, path string) error {
	if err := inotifyWatcher.Add(path, unix.IN_CREATE|unix.IN_DELETE|unix.IN_MOVE); err != nil {
		logger.Debug("failed to add inotify watch", zap.String("path", path), zap.Error(err))

		if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("failed to add inotify watch for %q: %w", path, err)
		}
	}

	logger.Debug("processing directory", zap.String("path", path))

	entries, err := os.ReadDir(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}

		return fmt.Errorf("failed to read directory %q: %w", path, err)
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name())

		st, err := os.Stat(fullPath)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}

			return fmt.Errorf("failed to stat %q: %w", fullPath, err)
		}

		if st.IsDir() {
			if err = ctrl.handleDir(logger, inotifyWatcher, detectedSymlinks, fullPath); err != nil {
				return err
			}
		} else {
			dest, err := os.Readlink(fullPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) || errors.Is(err, unix.EINVAL) {
					continue
				}

				return fmt.Errorf("failed to readlink %q: %w", fullPath, err)
			}

			if !strings.HasPrefix(entry.Name(), tempSymlinkPrefix) {
				detectedSymlinks[fullPath] = dest
			}
		}
	}

	return nil
}
