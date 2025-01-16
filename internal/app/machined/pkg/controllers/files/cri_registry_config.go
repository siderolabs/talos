// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/siderolabs/gen/optional"
	"github.com/siderolabs/gen/xslices"
	"go.uber.org/zap"

	"github.com/siderolabs/talos/internal/pkg/containers/cri/containerd"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
)

// CRIRegistryConfigController generates parts of the CRI config for registry configuration.
type CRIRegistryConfigController struct {
	bindMountCreated bool
}

// Name implements controller.Controller interface.
func (ctrl *CRIRegistryConfigController) Name() string {
	return "files.CRIRegistryConfigController"
}

// Inputs implements controller.Controller interface.
func (ctrl *CRIRegistryConfigController) Inputs() []controller.Input {
	return []controller.Input{
		{
			Namespace: cri.NamespaceName,
			Type:      cri.RegistriesConfigType,
			ID:        optional.Some(cri.RegistriesConfigID),
			Kind:      controller.InputWeak,
		},
	}
}

// Outputs implements controller.Controller interface.
func (ctrl *CRIRegistryConfigController) Outputs() []controller.Output {
	return []controller.Output{
		{
			Type: files.EtcFileSpecType,
			Kind: controller.OutputShared,
		},
	}
}

// Run implements controller.Controller interface.
//
//nolint:gocyclo
func (ctrl *CRIRegistryConfigController) Run(ctx context.Context, r controller.Runtime, _ *zap.Logger) error {
	basePath := filepath.Join(constants.CRIConfdPath, "hosts")
	shadowPath := filepath.Join(constants.SystemPath, basePath)

	// bind mount shadow path over to base path
	// shadow path is writeable, controller is going to update it
	// base path is read-only, containerd will read from it
	if !ctrl.bindMountCreated {
		if err := createBindMountDir(shadowPath, basePath); err != nil {
			return err
		}

		ctrl.bindMountCreated = true
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := safe.ReaderGetByID[*cri.RegistriesConfig](ctx, r, cri.RegistriesConfigID)
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting registries config: %w", err)
		}

		var (
			criRegistryContents []byte
			criHosts            *containerd.HostsConfig
		)

		if cfg != nil {
			criRegistryContents, err = containerd.GenerateCRIConfig(cfg.TypedSpec())
			if err != nil {
				return err
			}

			criHosts, err = containerd.GenerateHosts(cfg.TypedSpec(), basePath)
			if err != nil {
				return err
			}
		} else {
			criHosts = &containerd.HostsConfig{}
		}

		if err := safe.WriterModify(ctx, r, files.NewEtcFileSpec(files.NamespaceName, constants.CRIRegistryConfigPart),
			func(r *files.EtcFileSpec) error {
				spec := r.TypedSpec()

				spec.Contents = criRegistryContents
				spec.Mode = 0o600
				spec.SelinuxLabel = constants.EtcSelinuxLabel

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		if err := ctrl.syncHosts(shadowPath, criHosts); err != nil {
			return fmt.Errorf("error syncing hosts: %w", err)
		}

		r.ResetRestartBackoff()
	}
}

//nolint:gocyclo
func (ctrl *CRIRegistryConfigController) syncHosts(shadowPath string, criHosts *containerd.HostsConfig) error {
	// 1. create/update all files and directories
	for dirName, directory := range criHosts.Directories {
		path := filepath.Join(shadowPath, dirName)

		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}

		for _, file := range directory.Files {
			// match contents to see if the update can be skipped
			contents, err := os.ReadFile(filepath.Join(path, file.Name))
			if err == nil && bytes.Equal(contents, file.Contents) {
				continue
			}

			// write file
			if err = os.WriteFile(filepath.Join(path, file.Name), file.Contents, file.Mode); err != nil {
				return err
			}
		}

		// remove any files which shouldn't be present
		fileList, err := os.ReadDir(path)
		if err != nil {
			return err
		}

		fileListMap := xslices.ToSetFunc(fileList, fs.DirEntry.Name)

		for _, file := range directory.Files {
			delete(fileListMap, file.Name)
		}

		for file := range fileListMap {
			if err = os.Remove(filepath.Join(path, file)); err != nil {
				return err
			}
		}
	}

	// 2. remove any directories which shouldn't be present
	directoryList, err := os.ReadDir(shadowPath)
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
		if err = os.RemoveAll(filepath.Join(shadowPath, dirName)); err != nil {
			return err
		}
	}

	return nil
}
