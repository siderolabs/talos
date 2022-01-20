// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	hashiversion "github.com/hashicorp/go-version"

	"github.com/talos-systems/talos/pkg/machinery/extensions"
	"github.com/talos-systems/talos/pkg/version"
)

// Validate the extension: compatibility, contents, etc.
func (ext *Extension) Validate() error {
	if err := ext.validateConstraints(); err != nil {
		return err
	}

	if err := ext.validateContents(); err != nil {
		return err
	}

	return nil
}

func (ext *Extension) validateConstraints() error {
	if ext.Manifest.Metadata.Compatibility.Talos.Version != "" {
		talosVersion, err := hashiversion.NewVersion(version.Tag)
		if err != nil {
			return err
		}

		versionConstraint, err := hashiversion.NewConstraint(ext.Manifest.Metadata.Compatibility.Talos.Version)
		if err != nil {
			return fmt.Errorf("error parsing Talos version constraint: %w", err)
		}

		if !versionConstraint.Check(talosVersion) {
			return fmt.Errorf("version constraint %s can't be satisfied with Talos version %s", versionConstraint, talosVersion)
		}
	}

	return nil
}

//nolint:gocyclo
func (ext *Extension) validateContents() error {
	return filepath.WalkDir(ext.rootfsPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		itemPath, err := filepath.Rel(ext.rootfsPath, path)
		if err != nil {
			return err
		}

		itemPath = filepath.Join("/", itemPath)

		// check for -------w-
		if d.Type().Perm()&0o002 > 0 {
			return fmt.Errorf("world-writeable files are not allowed: %q", itemPath)
		}

		// no symlinks
		if d.Type().Type() == os.ModeSymlink {
			return fmt.Errorf("symlinks are not allowed: %q", itemPath)
		}

		var st fs.FileInfo

		st, err = d.Info()
		if err != nil {
			return err
		}

		// no hardlinks
		if !d.IsDir() && st.Sys().(*syscall.Stat_t).Nlink > 1 {
			return fmt.Errorf("hardlinks are not allowed: %q", itemPath)
		}

		// no special files
		if !d.IsDir() && !d.Type().IsRegular() {
			return fmt.Errorf("special files are not allowed: %q", itemPath)
		}

		// directories should be non-empty
		if d.IsDir() {
			var contents []fs.DirEntry

			contents, err = os.ReadDir(path)
			if err != nil {
				return err
			}

			if len(contents) == 0 {
				return fmt.Errorf("empty directories are not allowed: %q", itemPath)
			}
		}

		// regular file: check for file path being whitelisted
		if !d.IsDir() {
			dirPath := filepath.Dir(itemPath)

			allowed := false

			for _, allowedPath := range extensions.AllowedPaths {
				if strings.HasPrefix(dirPath, allowedPath) {
					_, err = filepath.Rel(allowedPath, dirPath)
					if err == nil {
						allowed = true

						break
					}
				}
			}

			if !allowed {
				return fmt.Errorf("path %q is not allowed in extensions", itemPath)
			}
		}

		return nil
	})
}
