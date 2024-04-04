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

	"github.com/blang/semver/v4"

	"github.com/siderolabs/talos/pkg/machinery/extensions"
	"github.com/siderolabs/talos/pkg/machinery/version"
)

// Validate the extension: compatibility, contents, etc.
func (ext *Extension) Validate() error {
	if err := ext.validateConstraints(); err != nil {
		return err
	}

	return ext.validateContents()
}

func (ext *Extension) validateConstraints() error {
	constraint := ext.Manifest.Metadata.Compatibility.Talos.Version

	if constraint != "" {
		talosVersion, err := semver.ParseTolerant(version.Tag)
		if err != nil {
			return err
		}

		versionConstraint, err := semver.ParseRange(trim(constraint))
		if err != nil {
			return fmt.Errorf("error parsing Talos version constraint: %w", err)
		}

		if !versionConstraint(coreVersion(talosVersion)) {
			return fmt.Errorf("version constraint %s can't be satisfied with Talos version %s", constraint, talosVersion)
		}
	}

	return nil
}

// trim removes 'v' symbol anywhere in string if it's located before the number.
func trim(constraint string) string {
	for i := 0; i < len(constraint); i++ {
		if constraint[i] == 'v' && i+1 < len(constraint) && constraint[i+1] >= '0' && constraint[i+1] <= '9' {
			constraint = constraint[:i] + constraint[i+1:]
		}
	}

	return constraint
}

func coreVersion(talosVersion semver.Version) semver.Version {
	return semver.Version{
		Major: talosVersion.Major,
		Minor: talosVersion.Minor,
		Patch: talosVersion.Patch,
	}
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

		// no special files
		if !d.IsDir() && !d.Type().IsRegular() && d.Type().Type() != os.ModeSymlink {
			return fmt.Errorf("special files are not allowed: %q", itemPath)
		}

		// regular file: check for file path being whitelisted
		if !d.IsDir() {
			allowed := false

			for _, allowedPath := range extensions.AllowedPaths {
				if strings.HasPrefix(itemPath, allowedPath) {
					_, err = filepath.Rel(allowedPath, itemPath)
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
