// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package extensions provides facilities for building initramfs.xz with extensions.
package extensions

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/internal/pkg/extensions"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	extinterface "github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

// Builder rebuilds initramfs.xz with extensions.
type Builder struct {
	// The initramfs will be rebuilt in-place.
	InitramfsPath string
	// Architecture of the initramfs.
	Arch string
	// ExtensionTreePath is a path to the extracted extension tree.
	ExtensionTreePath string
	// ExtensionValidateContents enables validation of the extension contents.
	ExtensionValidateContents bool
	// Printf is used for logging.
	Printf func(format string, v ...any)
	// Quirks for the Talos version being used.
	Quirks quirks.Quirks
}

// Build rebuilds the initramfs.xz with extensions.
//
//nolint:gocyclo
func (builder *Builder) Build() error {
	extensionsList, err := extensions.List(builder.ExtensionTreePath)
	if err != nil {
		return fmt.Errorf("error listing extensions: %w", err)
	}

	if len(extensionsList) == 0 {
		return nil
	}

	if err = builder.printExtensions(extensionsList); err != nil {
		return err
	}

	if err = builder.validateExtensions(extensionsList); err != nil {
		return err
	}

	extensionPathsWithKernelModules := findExtensionsWithKernelModules(extensionsList, builder.Quirks)

	if len(extensionPathsWithKernelModules) > 0 {
		var scratchPath string

		// create a temporary directory to store 'modules.dep' extension
		scratchPath, err = os.MkdirTemp("", "ext-modules")
		if err != nil {
			return err
		}

		defer os.RemoveAll(scratchPath) //nolint:errcheck

		kernelModuleDepExtension, genErr := extensions.GenerateKernelModuleDependencyTreeExtension(extensionPathsWithKernelModules, builder.InitramfsPath, scratchPath, builder.Quirks, builder.Printf)
		if genErr != nil {
			return genErr
		}

		extensionsList = append(extensionsList, kernelModuleDepExtension)
	}

	tempDir, err := os.MkdirTemp("", "ext")
	if err != nil {
		return err
	}

	defer os.RemoveAll(tempDir) //nolint:errcheck

	var cfg *extinterface.Config

	if cfg, err = builder.compressExtensions(extensionsList, tempDir); err != nil {
		return err
	}

	if err = cfg.Write(filepath.Join(tempDir, constants.ExtensionsConfigFile)); err != nil {
		return err
	}

	return builder.rebuildInitramfs(tempDir, builder.Quirks)
}

func (builder *Builder) validateExtensions(extensions []*extensions.Extension) error {
	builder.Printf("validating system extensions")

	opts := []extinterface.ValidationOption{
		extinterface.WithValidateConstraints(),
		extinterface.WithTalosVersion(builder.Quirks.Version()),
	}

	if builder.ExtensionValidateContents {
		opts = append(opts, extinterface.WithValidateContents())
	}

	for _, ext := range extensions {
		if err := ext.Validate(opts...); err != nil {
			return fmt.Errorf("error validating extension %q: %w", ext.Manifest.Metadata.Name, err)
		}
	}

	return nil
}

func (builder *Builder) compressExtensions(extensions []*extensions.Extension, tempDir string) (*extinterface.Config, error) {
	cfg := &extinterface.Config{}

	builder.Printf("compressing system extensions")

	for _, ext := range extensions {
		path, err := ext.Compress(tempDir, tempDir, builder.Quirks)
		if err != nil {
			return nil, fmt.Errorf("error compressing extension %q: %w", ext.Manifest.Metadata.Name, err)
		}

		cfg.Layers = append(cfg.Layers, &extinterface.Layer{
			Image:    filepath.Base(path),
			Metadata: ext.Manifest.Metadata,
		})
	}

	return cfg, nil
}
