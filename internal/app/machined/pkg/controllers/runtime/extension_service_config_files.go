// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime/extensions"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
	"github.com/siderolabs/talos/pkg/xfs"
	"github.com/siderolabs/talos/pkg/xfs/opendir"
)

// ExtensionServiceConfigFilesController writes down the config files for extension services.
type ExtensionServiceConfigFilesController struct {
	V1Alpha1Mode         v1alpha1runtime.Mode
	ExtensionsConfigRoot xfs.Root
}

// Name implements controller.Controller interface.
func (ctrl *ExtensionServiceConfigFilesController) Name() string {
	return "runtime.ExtensionServiceConfigFilesController"
}

// Inputs implements controller.Controller interface.
func (ctrl *ExtensionServiceConfigFilesController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: runtime.NamespaceName,
			Type:      runtime.ExtensionServiceConfigType,
			Kind:      controller.InputStrong,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *ExtensionServiceConfigFilesController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: runtime.ExtensionServiceConfigStatusType,
			Kind: controller.OutputExclusive,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *ExtensionServiceConfigFilesController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	if ctrl.V1Alpha1Mode == v1alpha1runtime.ModeContainer {
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		list, err := safe.ReaderListAll[*runtime.ExtensionServiceConfig](ctx, r)
		if err != nil {
			return fmt.Errorf("error listing extension services config: %w", err)
		}

		r.StartTrackingOutputs()

		// every path to keep in the root, and whether it is a directory
		wanted := map[string]bool{}

		for res := range list.All() {
			wanted[res.Metadata().ID()] = true

			for _, file := range res.TypedSpec().Files {
				fileName, err := extensions.ConfigFileName(res.Metadata().ID(), file.MountPath)
				if err != nil {
					return err
				}

				fileName = filepath.Join(res.Metadata().ID(), fileName)

				for dir := filepath.Dir(fileName); dir != "."; dir = filepath.Dir(dir) {
					wanted[dir] = true
				}

				wanted[fileName] = false
			}
		}

		// clean up first, so that a path which changed between a file and a directory can be written
		if err = cleanupExtensionServiceConfigFiles(ctrl.ExtensionsConfigRoot, wanted); err != nil {
			return err
		}

		for res := range list.All() {
			if err = ctrl.writeConfigFiles(res); err != nil {
				return err
			}
		}

		for res := range list.All() {
			if err = safe.WriterModify(ctx, r, runtime.NewExtensionServiceConfigStatusSpec(runtime.NamespaceName, res.Metadata().ID()), func(spec *runtime.ExtensionServiceConfigStatus) error {
				spec.TypedSpec().SpecVersion = res.Metadata().Version().String()

				return nil
			}); err != nil {
				return err
			}
		}

		if err = safe.CleanupOutputs[*runtime.ExtensionServiceConfigStatus](ctx, r); err != nil {
			return err
		}
	}
}

// writeConfigFiles writes the config files of an extension service through a root opened beneath
// its directory, so no mount path can resolve into another service's directory.
func (ctrl *ExtensionServiceConfigFilesController) writeConfigFiles(res *runtime.ExtensionServiceConfig) error {
	id := res.Metadata().ID()

	if err := xfs.MkdirAll(ctrl.ExtensionsConfigRoot, id, 0o755); err != nil {
		return fmt.Errorf("error creating directory %q: %w", id, err)
	}

	rootFd, err := ctrl.ExtensionsConfigRoot.Fd()
	if err != nil {
		return err
	}

	root := &xfs.UnixRoot{FS: opendir.NewBeneath(rootFd, id)}
	if err = root.OpenFS(); err != nil {
		return err
	}

	defer root.Close() //nolint:errcheck

	for _, file := range res.TypedSpec().Files {
		fileName, err := extensions.ConfigFileName(id, file.MountPath)
		if err != nil {
			return err
		}

		if err = xfs.MkdirAll(root, filepath.Dir(fileName), 0o755); err != nil {
			return fmt.Errorf("error creating directory for %q of extension %q: %w", fileName, id, err)
		}

		if err = updateFile(root, fileName, []byte(file.Content), 0o644); err != nil {
			return fmt.Errorf("error writing file %q of extension %q: %w", fileName, id, err)
		}
	}

	return nil
}

func cleanupExtensionServiceConfigFiles(root xfs.Root, wanted map[string]bool) error {
	return fs.WalkDir(root, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == "." {
			return err
		}

		if isDir, ok := wanted[path]; ok && isDir == d.IsDir() {
			return nil
		}

		if err = xfs.RemoveAll(root, path); err != nil {
			return err
		}

		if d.IsDir() {
			return fs.SkipDir
		}

		return nil
	})
}
