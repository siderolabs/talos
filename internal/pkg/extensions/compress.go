// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Compress builds the squashfs image in the specified destination folder.
func (ext *Extension) Compress(destinationPath string) (string, error) {
	destinationPath = filepath.Join(destinationPath, fmt.Sprintf("%s.sqsh", ext.directory))

	cmd := exec.Command("mksquashfs", ext.rootfsPath, destinationPath, "-all-root", "-noappend", "-comp", "xz", "-Xdict-size", "100%", "-no-progress")
	cmd.Stderr = os.Stderr

	return destinationPath, cmd.Run()
}
