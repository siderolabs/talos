// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import "path/filepath"

// ArtifactsPath is a path to artifacts output directory (set during the build).
var ArtifactsPath = "default/"

// ArtifactPath returns path to the artifact by name.
func ArtifactPath(name string) string {
	return filepath.Join(ArtifactsPath, name)
}
