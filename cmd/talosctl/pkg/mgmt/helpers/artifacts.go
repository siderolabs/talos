// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package helpers provides helpers for talosctl.
package helpers

import (
	"path/filepath"

	"github.com/talos-systems/talos/pkg/machinery/gendata"
)

// ArtifactsPath is a path to artifacts output directory (set during the build).
var ArtifactsPath = gendata.ArtifactsPath

// ArtifactPath returns path to the artifact by name.
func ArtifactPath(name string) string {
	return filepath.Join(ArtifactsPath, name)
}
