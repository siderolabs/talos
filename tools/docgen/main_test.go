// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"path/filepath"
	"testing"
)

// This test exists mainly for easier debugging with debugger.
func TestProcessFile(t *testing.T) {
	inputFile := filepath.Join("..", "..", "pkg", "machinery", "config", "types", "v1alpha1", "v1alpha1_types.go")
	outputFile := filepath.Join(t.TempDir(), "out.go")
	schemaOutputFile := filepath.Join(t.TempDir(), "out.schema.json")
	versionTagFile := filepath.Join("..", "..", "pkg", "machinery", "gendata", "data", "tag")
	processFiles([]string{inputFile}, outputFile, schemaOutputFile, versionTagFile)
}
