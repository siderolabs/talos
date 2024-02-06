// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"go.uber.org/zap"

	v1alpha1runtime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// ExtensionServiceConfigFilesController writes down the config files for extension services.
type ExtensionServiceConfigFilesController struct {
	V1Alpha1Mode            v1alpha1runtime.Mode
	ExtensionsConfigBaseDir string
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
func (ctrl *ExtensionServiceConfigFilesController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
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

		touchedFiles := map[string]struct{}{}

		for iter := list.Iterator(); iter.Next(); {
			extensionConfigPath := filepath.Join(ctrl.ExtensionsConfigBaseDir, iter.Value().Metadata().ID())

			if err = os.MkdirAll(extensionConfigPath, 0o755); err != nil {
				return fmt.Errorf("error creating directory %q: %w", extensionConfigPath, err)
			}

			touchedFiles[extensionConfigPath] = struct{}{}

			for _, file := range iter.Value().TypedSpec().Files {
				fileName := filepath.Join(extensionConfigPath, strings.ReplaceAll(strings.TrimPrefix(file.MountPath, "/"), "/", "-"))

				if err = updateFile(fileName, []byte(file.Content), 0o644); err != nil {
					return fmt.Errorf("error writing file %q: %w", fileName, err)
				}

				touchedFiles[fileName] = struct{}{}
			}

			if err = safe.WriterModify(ctx, r, runtime.NewExtensionServiceConfigStatusSpec(runtime.NamespaceName, iter.Value().Metadata().ID()), func(spec *runtime.ExtensionServiceConfigStatus) error {
				spec.TypedSpec().SpecVersion = iter.Value().Metadata().Version().String()

				return nil
			}); err != nil {
				return err
			}
		}

		// remove all files not managed by us
		if err = filepath.WalkDir(ctrl.ExtensionsConfigBaseDir, func(path string, d fs.DirEntry, walkErr error) error {
			if _, ok := touchedFiles[path]; path != ctrl.ExtensionsConfigBaseDir && !ok {
				if err = os.RemoveAll(path); err != nil {
					return err
				}
			}

			return nil
		}); err != nil {
			return err
		}

		if err = safe.CleanupOutputs[*runtime.ExtensionServiceConfigStatus](ctx, r); err != nil {
			return err
		}
	}
}
