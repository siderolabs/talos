// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AlekSi/pointer"
	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"

	"github.com/talos-systems/talos/internal/pkg/containers/cri/containerd"
	"github.com/talos-systems/talos/pkg/machinery/constants"
	"github.com/talos-systems/talos/pkg/machinery/resources/config"
	"github.com/talos-systems/talos/pkg/machinery/resources/files"
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
			Namespace: config.NamespaceName,
			Type:      config.MachineConfigType,
			ID:        pointer.ToString(config.V1Alpha1ID),
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
func (ctrl *CRIRegistryConfigController) Run(ctx context.Context, r controller.Runtime, logger *zap.Logger) error {
	basePath := filepath.Join(constants.CRIConfdPath, "hosts")
	shadowPath := filepath.Join(constants.SystemPath, basePath)

	// bind mount shadow path over to base path
	// shadow path is writeable, controller is going to update it
	// base path is read-only, containerd will read from it
	if !ctrl.bindMountCreated {
		// create shadow path
		if err := os.MkdirAll(shadowPath, 0o700); err != nil {
			return err
		}

		if err := unix.Mount(shadowPath, basePath, "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
			return fmt.Errorf("failed to create bind mount for %s -> %s: %w", shadowPath, basePath, err)
		}

		ctrl.bindMountCreated = true
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-r.EventCh():
		}

		cfg, err := r.Get(ctx, resource.NewMetadata(config.NamespaceName, config.MachineConfigType, config.V1Alpha1ID, resource.VersionUndefined))
		if err != nil && !state.IsNotFoundError(err) {
			return fmt.Errorf("error getting config: %w", err)
		}

		var (
			criRegistryContents []byte
			criHosts            *containerd.HostsConfig
		)

		if cfg != nil {
			criRegistryContents, err = containerd.GenerateCRIConfig(cfg.(*config.MachineConfig).Config().Machine().Registries())
			if err != nil {
				return err
			}

			criHosts, err = containerd.GenerateHosts(cfg.(*config.MachineConfig).Config().Machine().Registries(), basePath)
			if err != nil {
				return err
			}
		} else {
			criHosts = &containerd.HostsConfig{}
		}

		if err := r.Modify(ctx, files.NewEtcFileSpec(files.NamespaceName, constants.CRIRegistryConfigPart),
			func(r resource.Resource) error {
				spec := r.(*files.EtcFileSpec).TypedSpec()

				spec.Contents = criRegistryContents
				spec.Mode = 0o600

				return nil
			}); err != nil {
			return fmt.Errorf("error modifying resource: %w", err)
		}

		if err := ctrl.syncHosts(shadowPath, criHosts); err != nil {
			return fmt.Errorf("error syncing hosts: %w", err)
		}
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

		fileListMap := make(map[string]struct{}, len(fileList))

		for _, file := range fileList {
			fileListMap[file.Name()] = struct{}{}
		}

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
