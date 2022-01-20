// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/machinery/extensions"
)

// Load extension from the filesystem.
//
// This performs initial validation of the extension file structure.
func Load(path string) (*Extension, error) {
	extension := &Extension{
		directory: filepath.Base(path),
	}

	items, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	for _, item := range items {
		switch item.Name() {
		case "manifest.yaml":
			if err = extension.loadManifest(filepath.Join(path, item.Name())); err != nil {
				return nil, err
			}
		case "rootfs":
			extension.rootfsPath = filepath.Join(path, item.Name())
		default:
			return nil, fmt.Errorf("unexpected file %q", item.Name())
		}
	}

	var zeroManifest extensions.Manifest

	if extension.Manifest == zeroManifest {
		return nil, fmt.Errorf("extension manifest is missing")
	}

	if extension.rootfsPath == "" {
		return nil, fmt.Errorf("extension rootfs is missing")
	}

	return extension, nil
}

func (ext *Extension) loadManifest(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close() //nolint:errcheck

	if err = yaml.NewDecoder(f).Decode(&ext.Manifest); err != nil {
		return err
	}

	if ext.Manifest.Version != "v1alpha1" {
		return fmt.Errorf("unsupported manifest version: %q", ext.Manifest.Version)
	}

	return nil
}
