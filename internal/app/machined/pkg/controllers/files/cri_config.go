// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/containers/cri/containerd"
	"github.com/siderolabs/talos/internal/pkg/toml"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/xfs"
)

const criRegistryConfigPart = "01-registries.part"

// CRIConfigController generates the merged CRI configuration and registry hosts.
type CRIConfigController struct {
	// EtcRoot is the root for /etc filesystem operations.
	EtcRoot xfs.Root
}

// Name implements controller.Controller interface.
func (ctrl *CRIConfigController) Name() string {
	return "files.CRIConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CRIConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: cri.NamespaceName,
			Type:      cri.RegistriesConfigType,
			ID:        optional.Some(cri.RegistriesConfigID),
			Kind:      controller.InputWeak,
		},
		{
			Namespace: cri.NamespaceName,
			Type:      cri.CustomizationConfigType,
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CRIConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.EtcFileSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
func (ctrl *CRIConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	src := filepath.Join(constants.CRIConfdPath, "hosts")
	dest := filepath.Join("etc", src)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		if err := ctrl.reconcile(ctx, r, src, dest, logger); err != nil {
			return err
		}

		r.ResetRestartBackoff()
	}
}

func (ctrl *CRIConfigController) reconcile(
	ctx context.Context,
	r controller.Runtime,
	src string,
	dest string,
	logger *zap.Logger,
) error {
	registriesConfig, err := safe.ReaderGetByID[*cri.RegistriesConfig](ctx, r, cri.RegistriesConfigID)
	if err != nil {
		if state.IsNotFoundError(err) {
			return nil
		}

		return fmt.Errorf("error getting registries config: %w", err)
	}

	customizationConfigs, err := safe.ReaderListAll[*cri.CustomizationConfig](ctx, r)
	if err != nil {
		return fmt.Errorf("error listing customization configs: %w", err)
	}

	customizations := make(map[string]string, customizationConfigs.Len())

	for customization := range customizationConfigs.All() {
		customizations[customization.Metadata().ID()] = customization.TypedSpec().Content
	}

	if err = ctrl.renderConfig(ctx, r, registriesConfig.TypedSpec(), customizations); err != nil {
		return err
	}

	criHosts, err := containerd.GenerateHosts(registriesConfig.TypedSpec(), dest)
	if err != nil {
		return fmt.Errorf("error generating CRI registry hosts: %w", err)
	}

	if err = ctrl.syncHosts(src, criHosts, logger); err != nil {
		return fmt.Errorf("error syncing hosts: %w", err)
	}

	return nil
}

func (ctrl *CRIConfigController) renderConfig(
	ctx context.Context,
	r controller.Writer,
	registriesConfig *cri.RegistriesConfigSpec,
	customizations map[string]string,
) error {
	criRegistryContents, err := containerd.GenerateCRIConfig(registriesConfig)
	if err != nil {
		return fmt.Errorf("error generating CRI registry config: %w", err)
	}

	parts, err := ctrl.loadPhysicalParts()
	if err != nil {
		return fmt.Errorf("error loading physical CRI config parts: %w", err)
	}

	parts[criRegistryConfigPart] = toml.Part{
		Contents: criRegistryContents,
		Origin:   "in-memory registries",
	}

	for name, contents := range customizations {
		parts[fmt.Sprintf("20-%s.part", name)] = toml.Part{
			Contents: []byte(contents),
			Origin:   fmt.Sprintf("in-memory customization %q", name),
		}
	}

	merged, err := toml.Merge(parts)
	if err != nil {
		return fmt.Errorf("error merging CRI config: %w", err)
	}

	if err = safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, constants.CRIConfig),
		func(r *files.EtcFileSpec) error {
			for key := range r.Metadata().Annotations().Raw() {
				r.Metadata().Annotations().Delete(key)
			}

			spec := r.TypedSpec()

			spec.Contents = merged
			spec.Mode = 0o600
			spec.SelinuxLabel = constants.EtcSelinuxLabel

			return nil
		}); err != nil {
		return fmt.Errorf("error modifying resource: %w", err)
	}

	return nil
}

func (ctrl *CRIConfigController) loadPhysicalParts() (map[string]toml.Part, error) {
	entries, err := xfs.ReadDir(ctrl.EtcRoot, constants.CRIConfdPath)
	if err != nil {
		return nil, err
	}

	parts := make(map[string]toml.Part, len(entries)+1)

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".part") {
			continue
		}

		contents, err := xfs.ReadFile(ctrl.EtcRoot, filepath.Join(constants.CRIConfdPath, entry.Name()))
		if err != nil {
			return nil, err
		}

		parts[entry.Name()] = toml.Part{
			Contents: contents,
			Origin:   fmt.Sprintf("file %s", filepath.Join(constants.EtcCRIConfdPath, entry.Name())),
		}
	}

	return parts, nil
}

//nolint:gocyclo
func (ctrl *CRIConfigController) syncHosts(basePath string, criHosts *containerd.HostsConfig, _ *zap.Logger) error {
	// 1. create/update all files and directories
	for dirName, directory := range criHosts.Directories {
		path := filepath.Join(basePath, dirName)

		if err := xfs.MkdirAll(ctrl.EtcRoot, path, 0o700); err != nil {
			return err
		}

		for _, file := range directory.Files {
			// match contents to see if the update can be skipped
			contents, err := xfs.ReadFile(ctrl.EtcRoot, filepath.Join(path, file.Name))
			if err == nil && bytes.Equal(contents, file.Contents) {
				continue
			}

			// write file
			if err = xfs.WriteFile(
				ctrl.EtcRoot,
				filepath.Join(path, file.Name),
				file.Contents,
				file.Mode,
			); err != nil {
				return err
			}
		}

		// remove any files which shouldn't be present
		fileList, err := xfs.ReadDir(ctrl.EtcRoot, path)
		if err != nil {
			return err
		}

		fileListMap := xslices.ToSetFunc(fileList, fs.DirEntry.Name)

		for _, file := range directory.Files {
			delete(fileListMap, file.Name)
		}

		for file := range fileListMap {
			if err = xfs.Remove(ctrl.EtcRoot, filepath.Join(path, file)); err != nil {
				return err
			}
		}
	}

	// 2. remove any directories which shouldn't be present
	directoryList, err := xfs.ReadDir(ctrl.EtcRoot, basePath)
	if err != nil {
		return err
	}

	directoryListMap := make(map[string]struct{}, len(directoryList))

	for _, dir := range directoryList {
		directoryListMap[dir.Name()] = struct{}{}
	}

	for dirName := range criHosts.Directories {
		delete(directoryListMap, dirName)
	}

	for dirName := range directoryListMap {
		if err = xfs.RemoveAll(ctrl.EtcRoot, filepath.Join(basePath, dirName)); err != nil {
			return err
		}
	}

	return nil
}
